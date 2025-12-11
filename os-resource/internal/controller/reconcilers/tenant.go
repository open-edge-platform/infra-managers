// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package reconcilers

import (
	"context"
	"strings"
	"sync/atomic"

	osv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	tenant_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/tenant/v1"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/common"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/fsclient"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/invclient"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/util"
	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

const (
	loggerName = "TenantReconciler"
)

var (
	zlogTenant = logging.GetLogger(loggerName)
	// periodicReconciliationFlag indicates whether the current reconciliation is periodic (from ticker)
	// or event-driven (from watcher or initial startup).
	// Uses atomic.Bool for thread-safe access across goroutines.
	periodicReconciliationFlag atomic.Bool
)

type TenantReconciler struct {
	invClient *invclient.InventoryClient
	osConfig  common.OsConfig
}

func NewTenantReconciler(c *invclient.InventoryClient, osConf common.OsConfig) *TenantReconciler {
	return &TenantReconciler{
		invClient: c,
		osConfig:  osConf,
	}
}

func (tr *TenantReconciler) Reconcile(ctx context.Context,
	request rec_v2.Request[ReconcilerID],
) rec_v2.Directive[ReconcilerID] {
	tenantID, resourceID := UnwrapReconcilerID(request.ID)

	zlogTenant.Info().Msgf("Reconciling Tenant, tenantID=%s resourceID=%s", tenantID, resourceID)

	tenant, err := tr.invClient.GetTenantByResourceID(ctx, tenantID, resourceID)
	if directive := HandleInventoryError(err, request); directive != nil {
		return directive
	}

	err = tr.reconcileTenant(ctx, tenant)
	if directive := HandleInventoryError(err, request); directive != nil {
		return directive
	}

	return request.Ack()
}

// SetPeriodicReconciliationFlag sets the flag indicating whether reconciliation is periodic.
// This should be called by the controller before reconciliation begins.
func SetPeriodicReconciliationFlag(isPeriodic bool) {
	periodicReconciliationFlag.Store(isPeriodic)
}

// isPeriodicReconciliation checks if this reconciliation is part of the periodic tick cycle.
// Returns true for periodic reconciliation (from ticker), false for event-driven reconciliation
// (initial startup or watcher events).
func isPeriodicReconciliation(_ context.Context) bool {
	return periodicReconciliationFlag.Load()
}

func (tr *TenantReconciler) ackOsWatcherIfNeeded(
	ctx context.Context,
	tenant *tenant_v1.Tenant,
) error {
	zlogTenant.Info().Msgf("[ACK] ackOsWatcherIfNeeded called for tenant=%s resourceID=%s (current WatcherOsmanager=%v)",
		tenant.GetTenantId(), tenant.GetResourceId(), tenant.GetWatcherOsmanager())

	if tenant.GetWatcherOsmanager() {
		zlogTenant.Info().Msgf("[ACK] WatcherOsmanager already set for tenant=%s, skipping", tenant.GetTenantId())
		return nil
	}

	zlogTenant.Info().Msgf("[ACK] Setting WatcherOsmanager flag for tenant=%s resourceID=%s",
		tenant.GetTenantId(), tenant.GetResourceId())

	err := tr.invClient.UpdateTenantOSWatcher(ctx, tenant.GetTenantId(), tenant.GetResourceId(), true)
	if err != nil {
		zlogTenant.Error().Err(err).Msgf(
			"[ACK] FAILED to set WatcherOsmanager for tenant=%s (UpdateTenantOSWatcher returned error)",
			tenant.GetTenantId())
		return err
	}

	zlogTenant.Info().Msgf("[ACK] Successfully set WatcherOsmanager flag for tenant=%s", tenant.GetTenantId())
	return nil
}

func (tr *TenantReconciler) createNewOSResourceFromOSProfile(
	ctx context.Context, tenantID string, osProfile *fsclient.OSProfileManifest,
) (string, error) {
	osRes, err := util.ConvertOSProfileToOSResource(osProfile)
	if err != nil {
		return "", err
	}

	osRes.TenantId = tenantID

	// FIXME: ITEP-22977 remove this check when enforcing JSON-encoded string for `installedPackages`
	// retrieve package manifest and existing CVEs JSON contnet only for IMMUTABLE OS
	if osProfile.Spec.Type == "OS_TYPE_IMMUTABLE" {
		osRes.InstalledPackages, err = fsclient.GetPackageManifest(ctx, osProfile.Spec.OsPackageManifestURL)
		if err != nil {
			return "", err
		}
	}

	// Defer heavy CVE operations to periodic reconciliation to avoid blocking startup
	if isPeriodicReconciliation(ctx) {
		osRes.ExistingCves, err = fsclient.GetExistingCVEs(ctx, osProfile.Spec.Type, osProfile.Spec.OsExistingCvesURL)
		if err != nil {
			zlogTenant.Warn().Err(err).Msgf("Failed to fetch existing CVEs from URL: %s", osProfile.Spec.OsExistingCvesURL)
		}
		osRes.ExistingCvesUrl = osProfile.Spec.OsExistingCvesURL
		osRes.FixedCves, err = fsclient.GetFixedCVEs(ctx, osProfile.Spec.Type, osProfile.Spec.OsFixedCvesURL)
		if err != nil {
			zlogTenant.Warn().Err(err).Msgf("Failed to fetch fixed CVEs from URL: %s", osProfile.Spec.OsFixedCvesURL)
		}
		osRes.FixedCvesUrl = osProfile.Spec.OsFixedCvesURL
	} else {
		// During initial startup/event-driven reconciliation, set URLs but defer CVE content fetch
		osRes.ExistingCvesUrl = osProfile.Spec.OsExistingCvesURL
		osRes.FixedCvesUrl = osProfile.Spec.OsFixedCvesURL
		zlogTenant.Debug().Msgf("Deferring CVE fetch for new OS resource %s %s to periodic reconciliation",
			osProfile.Spec.ProfileName, osProfile.Spec.OsImageVersion)
	}

	return tr.invClient.CreateOSResource(ctx, tenantID, osRes)
}

func (tr *TenantReconciler) updateOSResourceFromOSProfile(
	ctx context.Context, tenantID string, osRes *osv1.OperatingSystemResource, osProfile *fsclient.OSProfileManifest,
) error {
	var err error
	var existingCVEs string

	// Only perform heavy CVE operations during periodic reconciliation
	if !isPeriodicReconciliation(ctx) {
		zlogTenant.Debug().Msgf("Deferring CVE update for OS resource %s %s to periodic reconciliation",
			osProfile.Spec.ProfileName, osProfile.Spec.OsImageVersion)
		return nil
	}

	existingCVEs, err = fsclient.GetExistingCVEs(ctx, osProfile.Spec.Type, osProfile.Spec.OsExistingCvesURL)
	if err != nil {
		zlogTenant.Warn().Err(err).Msgf("Failed to fetch existing CVEs from URL: %s", osProfile.Spec.OsExistingCvesURL)
	} else {
		// Compare existing CVEs and if they match doesn't match, update the OS resource with new existing CVEs.
		if strings.Compare(existingCVEs, osRes.ExistingCves) != 0 {
			zlogTenant.Info().Msgf("Existing CVEs differ for tenant %s, profile %s - updating from %d to %d characters",
				tenantID, osProfile.Spec.ProfileName, len(osRes.ExistingCves), len(existingCVEs))
			osRes.ExistingCves = existingCVEs

			err = tr.invClient.UpdateOSResourceExistingCves(ctx, tenantID, osRes)
			if err != nil {
				return err
			}
		} else {
			zlogTenant.Info().Msgf("Existing CVEs match for tenant %s, profile %s - no update needed",
				tenantID, osProfile.Spec.ProfileName)
		}
	}

	fixedCVEs, err := fsclient.GetFixedCVEs(ctx, osProfile.Spec.Type, osProfile.Spec.OsFixedCvesURL)
	if err != nil {
		zlogTenant.Warn().Err(err).Msgf("Failed to fetch fixed CVEs from URL: %s", osProfile.Spec.OsFixedCvesURL)
	} else {
		// Compare fixed CVEs and if they match doesn't match, update the OS resource with new fixed CVEs.
		if strings.Compare(fixedCVEs, osRes.FixedCves) != 0 {
			zlogTenant.Info().Msgf("Fixed CVEs differ for tenant %s, profile %s - updating from %d to %d characters",
				tenantID, osProfile.Spec.ProfileName, len(osRes.FixedCves), len(fixedCVEs))
			osRes.FixedCves = fixedCVEs

			err = tr.invClient.UpdateOSResourceFixedCves(ctx, tenantID, osRes)
			if err != nil {
				return err
			}
		} else {
			zlogTenant.Info().Msgf("Fixed CVEs match for tenant %s, profile %s - no update needed",
				tenantID, osProfile.Spec.ProfileName)
		}
	}

	return nil
}

//nolint:cyclop // cyclomatic complexity is 11
func (tr *TenantReconciler) initializeProviderIfNeeded(
	ctx context.Context, tenant *tenant_v1.Tenant, allOsProfiles map[string][]*fsclient.OSProfileManifest,
) error {
	if *common.DisableProviderAutomationFlag {
		zlogTenant.Debug().Msgf("Provider auto-creation disabled by feature flag")
		return nil
	}

	zlogTenant.Info().Msgf("Creating Provider for tenant %s with autoProvision=%v and default profile %q",
		tenant.GetTenantId(), tr.osConfig.AutoProvision, tr.osConfig.DefaultProfile)

	if tenant.GetWatcherOsmanager() {
		zlogTenant.Debug().Msgf("Tenant is already acknowledged, skipping auto-creation.")
		return nil
	}

	provider, err := tr.invClient.GetProviderSingularByName(ctx, tenant.GetTenantId(), util.InfraOnboardingProviderName)
	if err != nil && !inv_errors.IsNotFound(err) {
		return err
	}

	if provider != nil {
		zlogTenant.Debug().Msgf("Tenant is already initialized with Provider, skipping auto-creation.")
		return nil
	}

	defaultOSResourceID := ""
	if tr.osConfig.AutoProvision {
		defaultOSProfile, exists := allOsProfiles[tr.osConfig.DefaultProfile]
		if !exists || len(defaultOSProfile) == 0 {
			errMsg := inv_errors.Errorf("Default profile %s is not included in the list of OS profiles",
				tr.osConfig.DefaultProfile)
			zlogTenant.Error().Err(errMsg).Msg("")
			return errMsg
		}

		defaultOSResourceID, err = tr.invClient.FindOSResourceID(ctx, tenant.GetTenantId(),
			defaultOSProfile[0].Spec.ProfileName, defaultOSProfile[0].Spec.OsImageVersion)
		if err != nil {
			zlogTenant.Error().Err(err).Msgf("Cannot find OS resource ID based on profile name %s"+
				"and OS image version %s", defaultOSProfile[0].Spec.ProfileName, defaultOSProfile[0].Spec.OsImageVersion)
			return err
		}
	}

	providerRes, err := util.GetOnboardingProviderResource(tenant.GetTenantId(), defaultOSResourceID,
		tr.osConfig.AutoProvision, tr.osConfig.OSSecurityFeatureEnable)
	if err != nil {
		return err
	}

	err = tr.invClient.CreateProvider(ctx, tenant.GetTenantId(), providerRes)
	if err != nil && !inv_errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

//nolint:cyclop // cyclomatic complexity is 11
func (tr *TenantReconciler) reconcileTenant(
	ctx context.Context,
	tenant *tenant_v1.Tenant,
) error {
	zlogTenant.Info().Msgf(
		"[RECONCILE-START] reconcileTenant for tenant=%s resourceID=%s (CurrentState=%v, DesiredState=%v, WatcherOsmanager=%v)",
		tenant.GetTenantId(), tenant.GetResourceId(), tenant.GetCurrentState(), tenant.GetDesiredState(),
		tenant.GetWatcherOsmanager())

	if tenant.GetDesiredState() == tenant_v1.TenantState_TENANT_STATE_CREATED {
		osProfiles, err := fsclient.GetLatestOsProfiles(ctx, tr.osConfig.EnabledProfiles, tr.osConfig.OsProfileRevision)
		if err != nil {
			return err
		}

		osResources, err := tr.invClient.ListOSResourcesForTenant(ctx, tenant.GetTenantId())
		if err != nil {
			return err
		}

		// create a map from "profile ID" to OS resource to avoid expensive inner loop to check if OS resource already exists.
		// Profile ID is a unique identifier of OS profile and OS resource.
		// It is composed of the profile name and OS image version.
		mapProfileIDToOSResource := make(map[string]*osv1.OperatingSystemResource)
		for _, osRes := range osResources {
			osResProfileID := osRes.GetProfileName() + osRes.GetImageId()
			mapProfileIDToOSResource[osResProfileID] = osRes
		}

		for _, enabledProfile := range osProfiles {
			for _, osProfile := range enabledProfile {
				profileID := osProfile.Spec.ProfileName + osProfile.Spec.OsImageVersion

				_, exists := mapProfileIDToOSResource[profileID]
				if exists {
					zlogTenant.Debug().Msgf("OS resource %s %s already exists",
						osProfile.Spec.ProfileName, osProfile.Spec.OsImageVersion)

					// OS resource for given OS profile exists, update it
					err = tr.updateOSResourceFromOSProfile(ctx, tenant.GetTenantId(),
						mapProfileIDToOSResource[profileID], osProfile)
					if err != nil {
						zlogTenant.Warn().Err(err).Msgf(
							"Failed to update OS resource from profile: %s", osProfile.Spec.ProfileName)
						return err
					}

					continue
				}

				zlogTenant.Debug().Msgf("Creating OS resource %s %s",
					osProfile.Spec.ProfileName, osProfile.Spec.OsImageVersion)

				// OS resource for given OS profile doesn't exist, create it
				_, err = tr.createNewOSResourceFromOSProfile(ctx, tenant.GetTenantId(), osProfile)
				if err != nil {
					zlogTenant.Warn().Err(err).Msgf(
						"Failed to create new OS resource from profile: %s", osProfile.Spec.ProfileName)
					return err
				}
			}
		}

		err = tr.initializeProviderIfNeeded(ctx, tenant, osProfiles)
		if err != nil {
			return err
		}

		zlogTenant.Info().Msgf("[RECONCILE] Calling ackOsWatcherIfNeeded for tenant=%s", tenant.GetTenantId())
		err = tr.ackOsWatcherIfNeeded(ctx, tenant)
		if err != nil {
			zlogTenant.Error().Err(err).Msgf("[RECONCILE] ackOsWatcherIfNeeded FAILED for tenant=%s, returning error",
				tenant.GetTenantId())
			return err
		}
		zlogTenant.Info().Msgf("[RECONCILE] ackOsWatcherIfNeeded succeeded for tenant=%s", tenant.GetTenantId())
	}

	if tenant.GetDesiredState() == tenant_v1.TenantState_TENANT_STATE_DELETED {
		// no need to perform any cleanup, let Tenant Controller proceed
		if err := tr.invClient.UpdateTenantOSWatcher(
			ctx, tenant.GetTenantId(), tenant.GetResourceId(), false); err != nil {
			return err
		}
	}

	zlogTenant.Info().Msgf("[RECONCILE-END] reconcileTenant completed successfully for tenant=%s", tenant.GetTenantId())
	return nil
}
