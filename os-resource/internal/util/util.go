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
	platformBundleData, err := json.Marshal(&osProfile.Spec.PlatformBundle)
	if err != nil {
		zlogUtil.Error().Err(err).Msg("")
		return nil, inv_errors.Errorf("Failed to convert OS platform bundle for profile %s", osProfile.Spec.ProfileName)
	}

	return &osv1.OperatingSystemResource{
		Name:            osProfile.Spec.Name,
		ImageUrl:        osProfile.Spec.OsImageURL,
		ImageId:         osProfile.Spec.OsImageVersion,
		Sha256:          osProfile.Spec.OsImageSha256,
		Architecture:    osProfile.Spec.Architecture,
		ProfileName:     osProfile.Spec.ProfileName,
		SecurityFeature: osv1.SecurityFeature(osv1.SecurityFeature_value[osProfile.Spec.SecurityFeature]),
		OsType:          osv1.OsType(osv1.OsType_value[osProfile.Spec.Type]),
		OsProvider:      osv1.OsProviderKind(osv1.OsProviderKind_value[osProfile.Spec.Provider]),
		PlatformBundle:  string(platformBundleData),
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
