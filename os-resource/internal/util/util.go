// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package util provides utility functions for OS resource management.
//
//nolint:revive // Package name 'util' is intentional for utility functions
package util

import (
	"encoding/json"

	osv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	providerv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/provider/v1"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/providerconfiguration"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/fsclient"
)

const (
	loggerName = "OSRM-Util"
	// InfraOnboardingProviderName is the name of the infrastructure onboarding provider.
	InfraOnboardingProviderName = "infra_onboarding"
)

var zlogUtil = logging.GetLogger(loggerName)

// ConvertOSProfileToOSResource converts an OS profile to an OS resource.
//
//nolint:cyclop // complexity is 11 due to extensive validation
func ConvertOSProfileToOSResource(osProfile *fsclient.OSProfileManifest) (*osv1.OperatingSystemResource, error) {
	platformBundle := ""
	metadata := ""

	if len(osProfile.Spec.PlatformBundle) != 0 {
		pbData, err := json.Marshal(&osProfile.Spec.PlatformBundle)
		if err != nil {
			zlogUtil.Error().Err(err).Msg("")
			return nil, inv_errors.Errorf("Failed to convert OS platform bundle for profile %s", osProfile.Spec.ProfileName)
		}
		platformBundle = string(pbData)
	}

	if len(osProfile.Metadata) != 0 {
		mdData, err := json.Marshal(&osProfile.Metadata)
		if err != nil {
			zlogUtil.Error().Err(err).Msg("")
			return nil, inv_errors.Errorf("Failed to convert OS metadata for profile %s", osProfile.Spec.ProfileName)
		}
		metadata = string(mdData)
	}

	_osType, valid := osv1.OsType_value[osProfile.Spec.Type]
	if !valid {
		osTypeErr := inv_errors.Errorf("Invalid OS type: %s", osProfile.Spec.Type)
		zlogUtil.Error().Err(osTypeErr).Msg("")
		return nil, osTypeErr
	}

	osType := osv1.OsType(_osType)

	if osType == osv1.OsType_OS_TYPE_UNSPECIFIED {
		osTypeErr := inv_errors.Errorf("OS type cannot be %s", osType.String())
		zlogUtil.Error().Err(osTypeErr).Msg("")
		return nil, osTypeErr
	}

	_osProvider, valid := osv1.OsProviderKind_value[osProfile.Spec.Provider]
	if !valid {
		osProviderErr := inv_errors.Errorf("Invalid OS provider: %s", osProfile.Spec.Provider)
		zlogUtil.Error().Err(osProviderErr).Msg("")
		return nil, osProviderErr
	}

	osProvider := osv1.OsProviderKind(_osProvider)

	if osProvider == osv1.OsProviderKind_OS_PROVIDER_KIND_UNSPECIFIED {
		osProviderErr := inv_errors.Errorf("OS provider cannot be %s", osProvider.String())
		zlogUtil.Error().Err(osProviderErr).Msg("")
		return nil, osProviderErr
	}

	_securityFeature, valid := osv1.SecurityFeature_value[osProfile.Spec.SecurityFeature]
	if !valid {
		securityFeaturErr := inv_errors.Errorf("Invalid security feature: %s", osProfile.Spec.SecurityFeature)
		zlogUtil.Error().Err(securityFeaturErr).Msg("")
		return nil, securityFeaturErr
	}

	securityFeature := osv1.SecurityFeature(_securityFeature)

	if securityFeature == osv1.SecurityFeature_SECURITY_FEATURE_UNSPECIFIED {
		securityFeaturErr := inv_errors.Errorf("Security feature cannot be %s", securityFeature.String())
		zlogUtil.Error().Err(securityFeaturErr).Msg("")
		return nil, securityFeaturErr
	}

	return &osv1.OperatingSystemResource{
		Name:            osProfile.Spec.Name,
		Metadata:        metadata,
		ImageUrl:        osProfile.Spec.OsImageURL,
		ImageId:         osProfile.Spec.OsImageVersion,
		Sha256:          osProfile.Spec.OsImageSha256,
		Architecture:    osProfile.Spec.Architecture,
		ProfileName:     osProfile.Spec.ProfileName,
		SecurityFeature: osv1.SecurityFeature(osv1.SecurityFeature_value[osProfile.Spec.SecurityFeature]),
		OsType:          osType,
		OsProvider:      osProvider,
		PlatformBundle:  platformBundle,
		Description:     osProfile.Spec.Description,
		TlsCaCert:       osProfile.Spec.TLSCaCert,
	}, nil
}

// GetOnboardingProviderResource retrieves the onboarding provider resource.
func GetOnboardingProviderResource(
	tenantID, defaultOSResourceID string, autoProvision bool, defaultSecurityFlag bool,
) (*providerv1.ProviderResource, error) {
	config := providerconfiguration.ProviderConfig{
		AutoProvision: autoProvision,
	}

	if autoProvision {
		config.DefaultOs = defaultOSResourceID
	}
	config.OSSecurityFeatureEnable = defaultSecurityFlag
	configData, err := json.Marshal(&config)
	if err != nil {
		invErr := inv_errors.Errorf("Cannot generate ProviderConfig: %v", err)
		zlogUtil.Error().Err(invErr).Msg("")
		return nil, invErr
	}

	return &providerv1.ProviderResource{
		ProviderKind:   providerv1.ProviderKind_PROVIDER_KIND_BAREMETAL,
		ProviderVendor: providerv1.ProviderVendor_PROVIDER_VENDOR_UNSPECIFIED,
		Name:           InfraOnboardingProviderName,
		ApiEndpoint:    "null",
		Config:         string(configData),
		TenantId:       tenantID,
	}, nil
}
