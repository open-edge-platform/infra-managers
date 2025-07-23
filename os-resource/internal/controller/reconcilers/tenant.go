// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package reconcilers

import (
	"context"

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

const loggerName = "TenantReconciler"

var zlogTenant = logging.GetLogger(loggerName)

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

func (tr *TenantReconciler) ackOsWatcherIfNeeded(
	ctx context.Context,
	tenant *tenant_v1.Tenant,
) error {
	if tenant.GetWatcherOsmanager() {
		zlogTenant.Debug().Msgf("Skipping acknowledging the OS watcher as it's already set")
		return nil
	}

	return tr.invClient.UpdateTenantOSWatcher(ctx, tenant.GetTenantId(), tenant.GetResourceId(), true)
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
		var err error
		osRes.InstalledPackages, err = fsclient.GetPackageManifest(ctx, osProfile.Spec.OsPackageManifestURL)
		if err != nil {
			return "", err
		}
		osRes.ExistingCves, err = fsclient.GetExistingCVEs(ctx, osProfile.Spec.OsExistingCvesURL)
		if err != nil {
			return "", err
		}
		osRes.ExistingCvesUrl = osProfile.Spec.OsExistingCvesURL
		osRes.FixedCves, err = fsclient.GetFixedCVEs(ctx, osProfile.Spec.OsFixedCvesURL)
		if err != nil {
			// Fixed CVEs list is not getting published as of now, but it will be supported in future.
			// SO for now, when there is error in fetching fixed CVEs, do not return error, instead log it.
			zlogTenant.Warn().Err(err).Msgf("Failed to fetch fixed CVEs from URL: %s", osProfile.Spec.OsFixedCvesURL)
		}
		osRes.FixedCvesUrl = osProfile.Spec.OsFixedCvesURL
	}

	return tr.invClient.CreateOSResource(ctx, tenantID, osRes)
}

func (tr *TenantReconciler) updateOSResourceFromOSProfile(
	ctx context.Context, tenantID string, osRes *osv1.OperatingSystemResource, osProfile *fsclient.OSProfileManifest,
) error {
	if osProfile.Spec.Type == "OS_TYPE_IMMUTABLE" {
		var err error
		osRes.ExistingCves, err = fsclient.GetExistingCVEs(ctx, osProfile.Spec.OsExistingCvesURL)
		if err != nil {
			return err
		}
		osRes.ExistingCvesUrl = osProfile.Spec.OsExistingCvesURL

		err = tr.invClient.UpdateOSResourceExistingCvesAndURL(ctx, tenantID, osRes)
		if err != nil {
			return err
		}
	} else {
		// For mutable OS, we do not update the existing OS resource.
		zlogTenant.Debug().Msgf("Skipping update for mutable OS profile %s", osProfile.Spec.ProfileName)
	}
	return nil
}

//nolint:cyclop // cyclomatic complexity is 11
func (tr *TenantReconciler) initializeProviderIfNeeded(
	ctx context.Context, tenant *tenant_v1.Tenant, allOsProfiles map[string]*fsclient.OSProfileManifest,
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
		if !exists {
			errMsg := inv_errors.Errorf("Default profile %s is not included in the list of OS profiles",
				tr.osConfig.DefaultProfile)
			zlogTenant.Error().Err(errMsg).Msg("")
			return errMsg
		}

		defaultOSResourceID, err = tr.invClient.FindOSResourceID(ctx, tenant.GetTenantId(),
			defaultOSProfile.Spec.ProfileName, defaultOSProfile.Spec.OsImageVersion)
		if err != nil {
			zlogTenant.Error().Err(err).Msgf("Cannot find OS resource ID based on profile name %s"+
				"and OS image version %s", defaultOSProfile.Spec.ProfileName, defaultOSProfile.Spec.OsImageVersion)
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
	zlogTenant.Debug().Msgf("Reconciling tenant with resource ID %s, with Current state: %v, Desired state: %v.",
		tenant.GetResourceId(), tenant.GetCurrentState(), tenant.GetDesiredState())

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

		for _, osProfile := range osProfiles {
			profileID := osProfile.Spec.ProfileName + osProfile.Spec.OsImageVersion

			_, exists := mapProfileIDToOSResource[profileID]
			if exists {
				zlogTenant.Debug().Msgf("OS resource %s %s already exists",
					osProfile.Spec.ProfileName, osProfile.Spec.OsImageVersion)

				// OS resource for given OS profile exists, update it
				err = tr.updateOSResourceFromOSProfile(ctx, tenant.GetTenantId(), mapProfileIDToOSResource[profileID], osProfile)
				if err != nil {
					return err
				}

				continue
			}

			// OS resource for given OS profile doesn't exist, create it
			_, err = tr.createNewOSResourceFromOSProfile(ctx, tenant.GetTenantId(), osProfile)
			if err != nil {
				return err
			}
		}

		err = tr.initializeProviderIfNeeded(ctx, tenant, osProfiles)
		if err != nil {
			return err
		}

		err = tr.ackOsWatcherIfNeeded(ctx, tenant)
		if err != nil {
			return err
		}

		err = tr.updateInstancesIfNeeded(ctx, tenant.GetTenantId(), osProfiles)
		if err != nil {
			return err
		}
	}

	if tenant.GetDesiredState() == tenant_v1.TenantState_TENANT_STATE_DELETED {
		// no need to perform any cleanup, let Tenant Controller proceed
		if err := tr.invClient.UpdateTenantOSWatcher(
			ctx, tenant.GetTenantId(), tenant.GetResourceId(), false); err != nil {
			return err
		}
	}

	return nil
}

func (tr *TenantReconciler) updateInstancesIfNeeded(
	ctx context.Context, tenantID string, latestOSProfiles map[string]*fsclient.OSProfileManifest,
) error {
	if tr.osConfig.ManualMode {
		zlogTenant.Debug().Msgf("ManualMode set to true, Instances will not be auto-updated.")
		return nil
	}

	profileNameToOSResourceID := make(map[string]string)
	for _, osProfile := range latestOSProfiles {
		id, err := tr.invClient.FindOSResourceID(ctx, tenantID, osProfile.Spec.ProfileName, osProfile.Spec.OsImageVersion)
		if err != nil {
			return err
		}
		profileNameToOSResourceID[osProfile.Spec.ProfileName] = id
	}

	instances, err := tr.invClient.ListInstancesForTenant(ctx, tenantID)
	if err != nil {
		return err
	}

	for _, i := range instances {
		profileName := i.DesiredOs.GetProfileName()

		if i.DesiredOs.GetResourceId() != profileNameToOSResourceID[profileName] {
			err = tr.invClient.UpdateInstanceDesiredOS(ctx, tenantID, i.GetResourceId(),
				profileNameToOSResourceID[profileName])
			if err != nil {
				return err
			}
		}
	}

	return nil
}
