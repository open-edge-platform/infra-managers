// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

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

	InfraOnboardingProviderName = "infra_onboarding"
)

var zlogUtil = logging.GetLogger(loggerName)

func ConvertOSProfileToOSResource(osProfile *fsclient.OSProfileManifest) (*osv1.OperatingSystemResource, error) {
	platformBundle := ""

	if len(osProfile.Spec.PlatformBundle) != 0 {
		pbData, err := json.Marshal(&osProfile.Spec.PlatformBundle)
		if err != nil {
			zlogUtil.Error().Err(err).Msg("")
			return nil, inv_errors.Errorf("Failed to convert OS platform bundle for profile %s", osProfile.Spec.ProfileName)
		}
		platformBundle = string(pbData)
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
	}, nil
}

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
