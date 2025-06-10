// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	"reflect"
	"testing"

	osv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/fsclient"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/util"
)

//nolint:funlen // it's a test
func TestConvertOSProfileToOSResource(t *testing.T) {
	type args struct {
		osProfile *fsclient.OSProfileManifest
	}
	tests := []struct {
		name    string
		args    args
		want    *osv1.OperatingSystemResource
		wantErr bool
	}{
		{
			name: "Success_Mutable",
			args: args{
				//nolint:tagliatelle // must be in sync with OS profiles
				&fsclient.OSProfileManifest{
					AppVersion: "",
					Metadata: map[string]interface{}{
						"key1": "value1",
						"key2": "value2",
					},
					Spec: struct {
						Name                 string                 `yaml:"name"`
						Type                 string                 `yaml:"type"`
						Provider             string                 `yaml:"provider"`
						Architecture         string                 `yaml:"architecture"`
						ProfileName          string                 `yaml:"profileName"`
						OsImageURL           string                 `yaml:"osImageUrl"`
						OsImageSha256        string                 `yaml:"osImageSha256"`
						OsImageVersion       string                 `yaml:"osImageVersion"`
						OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
						SecurityFeature      string                 `yaml:"securityFeature"`
						PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
						Description          string                 `yaml:"description"`
					}{
						Name:            "test",
						Type:            "OS_TYPE_MUTABLE",
						Provider:        "OS_PROVIDER_KIND_INFRA",
						Architecture:    "arch",
						ProfileName:     "profile-test",
						OsImageURL:      "URL",
						OsImageSha256:   "sha256",
						OsImageVersion:  "1.0",
						SecurityFeature: "SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION",
						PlatformBundle:  nil,
						Description:     "test-description",
					},
				},
			},
			want: &osv1.OperatingSystemResource{
				Name:            "test",
				Metadata:        "{\"key1\":\"value1\",\"key2\":\"value2\"}",
				Architecture:    "arch",
				ImageUrl:        "URL",
				ImageId:         "1.0",
				Sha256:          "sha256",
				ProfileName:     "profile-test",
				SecurityFeature: osv1.SecurityFeature_SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION,
				OsType:          osv1.OsType_OS_TYPE_MUTABLE,
				OsProvider:      osv1.OsProviderKind_OS_PROVIDER_KIND_INFRA,
				PlatformBundle:  "",
				Description:     "test-description",
			},
			wantErr: false,
		},
		{
			name: "Success_WithPlatformBundle",
			args: args{
				//nolint:tagliatelle // must be in sync with OS profiles
				&fsclient.OSProfileManifest{
					AppVersion: "",
					Metadata:   map[string]interface{}{},
					Spec: struct {
						Name                 string                 `yaml:"name"`
						Type                 string                 `yaml:"type"`
						Provider             string                 `yaml:"provider"`
						Architecture         string                 `yaml:"architecture"`
						ProfileName          string                 `yaml:"profileName"`
						OsImageURL           string                 `yaml:"osImageUrl"`
						OsImageSha256        string                 `yaml:"osImageSha256"`
						OsImageVersion       string                 `yaml:"osImageVersion"`
						OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
						SecurityFeature      string                 `yaml:"securityFeature"`
						PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
						Description          string                 `yaml:"description"`
					}{
						Name:            "test",
						Type:            "OS_TYPE_MUTABLE",
						Provider:        "OS_PROVIDER_KIND_INFRA",
						Architecture:    "arch",
						ProfileName:     "profile-test",
						OsImageURL:      "URL",
						OsImageSha256:   "sha256",
						OsImageVersion:  "1.0",
						SecurityFeature: "SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION",
						PlatformBundle: map[string]interface{}{
							"artifactName": "artifact",
						},
						Description: "test-description",
					},
				},
			},
			want: &osv1.OperatingSystemResource{
				Name:            "test",
				Architecture:    "arch",
				ImageUrl:        "URL",
				ImageId:         "1.0",
				Sha256:          "sha256",
				ProfileName:     "profile-test",
				SecurityFeature: osv1.SecurityFeature_SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION,
				OsType:          osv1.OsType_OS_TYPE_MUTABLE,
				OsProvider:      osv1.OsProviderKind_OS_PROVIDER_KIND_INFRA,
				PlatformBundle:  "{\"artifactName\":\"artifact\"}",
				Description:     "test-description",
			},
			wantErr: false,
		},
		{
			name: "Success_EmptyPlatformBundle",
			args: args{
				//nolint:tagliatelle // must be in sync with OS profiles
				&fsclient.OSProfileManifest{
					AppVersion: "",
					Metadata:   map[string]interface{}{},
					Spec: struct {
						Name                 string                 `yaml:"name"`
						Type                 string                 `yaml:"type"`
						Provider             string                 `yaml:"provider"`
						Architecture         string                 `yaml:"architecture"`
						ProfileName          string                 `yaml:"profileName"`
						OsImageURL           string                 `yaml:"osImageUrl"`
						OsImageSha256        string                 `yaml:"osImageSha256"`
						OsImageVersion       string                 `yaml:"osImageVersion"`
						OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
						SecurityFeature      string                 `yaml:"securityFeature"`
						PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
						Description          string                 `yaml:"description"`
					}{
						Name:            "test",
						Type:            "OS_TYPE_MUTABLE",
						Provider:        "OS_PROVIDER_KIND_INFRA",
						Architecture:    "arch",
						ProfileName:     "profile-test",
						OsImageURL:      "URL",
						OsImageSha256:   "sha256",
						OsImageVersion:  "1.0",
						SecurityFeature: "SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION",
						PlatformBundle:  map[string]interface{}{},
						Description:     "test-description",
					},
				},
			},
			want: &osv1.OperatingSystemResource{
				Name:            "test",
				Architecture:    "arch",
				ImageUrl:        "URL",
				ImageId:         "1.0",
				Sha256:          "sha256",
				ProfileName:     "profile-test",
				SecurityFeature: osv1.SecurityFeature_SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION,
				OsType:          osv1.OsType_OS_TYPE_MUTABLE,
				OsProvider:      osv1.OsProviderKind_OS_PROVIDER_KIND_INFRA,
				PlatformBundle:  "",
				Description:     "test-description",
			},
			wantErr: false,
		},
		{
			name: "Failed_InvalidOSType",
			args: args{
				//nolint:tagliatelle // must be in sync with OS profiles
				&fsclient.OSProfileManifest{
					AppVersion: "",
					Metadata:   map[string]interface{}{},
					Spec: struct {
						Name                 string                 `yaml:"name"`
						Type                 string                 `yaml:"type"`
						Provider             string                 `yaml:"provider"`
						Architecture         string                 `yaml:"architecture"`
						ProfileName          string                 `yaml:"profileName"`
						OsImageURL           string                 `yaml:"osImageUrl"`
						OsImageSha256        string                 `yaml:"osImageSha256"`
						OsImageVersion       string                 `yaml:"osImageVersion"`
						OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
						SecurityFeature      string                 `yaml:"securityFeature"`
						PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
						Description          string                 `yaml:"description"`
					}{
						Name:            "test",
						Type:            "OS_TYPE_INVALID",
						Provider:        "OS_PROVIDER_KIND_INFRA",
						Architecture:    "arch",
						ProfileName:     "profile-test",
						OsImageURL:      "URL",
						OsImageSha256:   "sha256",
						OsImageVersion:  "1.0",
						SecurityFeature: "SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION",
						PlatformBundle:  map[string]interface{}{},
						Description:     "test-description",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Failed_OSTypeUnspecified",
			args: args{
				//nolint:tagliatelle // must be in sync with OS profiles
				&fsclient.OSProfileManifest{
					AppVersion: "",
					Metadata:   map[string]interface{}{},
					Spec: struct {
						Name                 string                 `yaml:"name"`
						Type                 string                 `yaml:"type"`
						Provider             string                 `yaml:"provider"`
						Architecture         string                 `yaml:"architecture"`
						ProfileName          string                 `yaml:"profileName"`
						OsImageURL           string                 `yaml:"osImageUrl"`
						OsImageSha256        string                 `yaml:"osImageSha256"`
						OsImageVersion       string                 `yaml:"osImageVersion"`
						OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
						SecurityFeature      string                 `yaml:"securityFeature"`
						PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
						Description          string                 `yaml:"description"`
					}{
						Name:            "test",
						Type:            "OS_TYPE_UNSPECIFIED",
						Provider:        "OS_PROVIDER_KIND_INFRA",
						Architecture:    "arch",
						ProfileName:     "profile-test",
						OsImageURL:      "URL",
						OsImageSha256:   "sha256",
						OsImageVersion:  "1.0",
						SecurityFeature: "SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION",
						PlatformBundle:  map[string]interface{}{},
						Description:     "test-description",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Failed_InvalidOSProvider",
			args: args{
				//nolint:tagliatelle // must be in sync with OS profiles
				&fsclient.OSProfileManifest{
					AppVersion: "",
					Metadata:   map[string]interface{}{},
					Spec: struct {
						Name                 string                 `yaml:"name"`
						Type                 string                 `yaml:"type"`
						Provider             string                 `yaml:"provider"`
						Architecture         string                 `yaml:"architecture"`
						ProfileName          string                 `yaml:"profileName"`
						OsImageURL           string                 `yaml:"osImageUrl"`
						OsImageSha256        string                 `yaml:"osImageSha256"`
						OsImageVersion       string                 `yaml:"osImageVersion"`
						OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
						SecurityFeature      string                 `yaml:"securityFeature"`
						PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
						Description          string                 `yaml:"description"`
					}{
						Name:            "test",
						Type:            "OS_TYPE_IMMUTABLE",
						Provider:        "OS_PROVIDER_KIND_INVALID",
						Architecture:    "arch",
						ProfileName:     "profile-test",
						OsImageURL:      "URL",
						OsImageSha256:   "sha256",
						OsImageVersion:  "1.0",
						SecurityFeature: "SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION",
						PlatformBundle:  map[string]interface{}{},
						Description:     "test-description",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Failed_OSProviderUnspecified",
			args: args{
				//nolint:tagliatelle // must be in sync with OS profiles
				&fsclient.OSProfileManifest{
					AppVersion: "",
					Metadata:   map[string]interface{}{},
					Spec: struct {
						Name                 string                 `yaml:"name"`
						Type                 string                 `yaml:"type"`
						Provider             string                 `yaml:"provider"`
						Architecture         string                 `yaml:"architecture"`
						ProfileName          string                 `yaml:"profileName"`
						OsImageURL           string                 `yaml:"osImageUrl"`
						OsImageSha256        string                 `yaml:"osImageSha256"`
						OsImageVersion       string                 `yaml:"osImageVersion"`
						OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
						SecurityFeature      string                 `yaml:"securityFeature"`
						PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
						Description          string                 `yaml:"description"`
					}{
						Name:            "test",
						Type:            "OS_TYPE_IMMUTABLE",
						Provider:        "OS_PROVIDER_KIND_UNSPECIFIED",
						Architecture:    "arch",
						ProfileName:     "profile-test",
						OsImageURL:      "URL",
						OsImageSha256:   "sha256",
						OsImageVersion:  "1.0",
						SecurityFeature: "SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION",
						PlatformBundle:  map[string]interface{}{},
						Description:     "test-description",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Failed_InvalidSecurityFeature",
			args: args{
				//nolint:tagliatelle // must be in sync with OS profiles
				&fsclient.OSProfileManifest{
					AppVersion: "",
					Metadata:   map[string]interface{}{},
					Spec: struct {
						Name                 string                 `yaml:"name"`
						Type                 string                 `yaml:"type"`
						Provider             string                 `yaml:"provider"`
						Architecture         string                 `yaml:"architecture"`
						ProfileName          string                 `yaml:"profileName"`
						OsImageURL           string                 `yaml:"osImageUrl"`
						OsImageSha256        string                 `yaml:"osImageSha256"`
						OsImageVersion       string                 `yaml:"osImageVersion"`
						OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
						SecurityFeature      string                 `yaml:"securityFeature"`
						PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
						Description          string                 `yaml:"description"`
					}{
						Name:            "test",
						Type:            "OS_TYPE_MUTABLE",
						Provider:        "OS_PROVIDER_KIND_INFRA",
						Architecture:    "arch",
						ProfileName:     "profile-test",
						OsImageURL:      "URL",
						OsImageSha256:   "sha256",
						OsImageVersion:  "1.0",
						SecurityFeature: "SECURITY_FEATURE_INVALID",
						PlatformBundle:  map[string]interface{}{},
						Description:     "test-description",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Failed_SecurityFeatureUnspecified",
			args: args{
				//nolint:tagliatelle // must be in sync with OS profiles
				&fsclient.OSProfileManifest{
					AppVersion: "",
					Metadata:   map[string]interface{}{},
					Spec: struct {
						Name                 string                 `yaml:"name"`
						Type                 string                 `yaml:"type"`
						Provider             string                 `yaml:"provider"`
						Architecture         string                 `yaml:"architecture"`
						ProfileName          string                 `yaml:"profileName"`
						OsImageURL           string                 `yaml:"osImageUrl"`
						OsImageSha256        string                 `yaml:"osImageSha256"`
						OsImageVersion       string                 `yaml:"osImageVersion"`
						OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
						SecurityFeature      string                 `yaml:"securityFeature"`
						PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
						Description          string                 `yaml:"description"`
					}{
						Name:            "test",
						Type:            "OS_TYPE_MUTABLE",
						Provider:        "OS_PROVIDER_KIND_INFRA",
						Architecture:    "arch",
						ProfileName:     "profile-test",
						OsImageURL:      "URL",
						OsImageSha256:   "sha256",
						OsImageVersion:  "1.0",
						SecurityFeature: "SECURITY_FEATURE_UNSPECIFIED",
						PlatformBundle:  map[string]interface{}{},
						Description:     "test-description",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := util.ConvertOSProfileToOSResource(tt.args.osProfile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertOSProfileToOSResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertOSProfileToOSResource() got = %v, want %v", got, tt.want)
			}
		})
	}
}
