// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	as "github.com/open-edge-platform/infra-core/inventory/v2/pkg/artifactservice"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/common"
)

const (
	EnProfileRepo = "files-edge-orch/en/manifest-en-profile/"
	UbuntuProfile = `# SPDX-FileCopyrightText: (C) 2024 Intel Corporation
appVersion: apps/v1
metadata:
  release: 25.04.0
  version: 0.1.0
spec:
  name: Ubuntu 22.04.5 LTS
  type: OS_TYPE_MUTABLE
  provider: OS_PROVIDER_KIND_INFRA
  architecture: x86_64
  profileName: ubuntu-22.04-lts-generic
  osImageUrl: https://cloud-images.ubuntu.com/releases/22.04/release-20241217/ubuntu-22.04-server-cloudimg-amd64.img
  osImageVersion: 22.04.5
  osImageSha256: 0d8345a343c2547e55ac815342e6cb4a593aa5556872651eb47e6856a2bb0cdd
  osExistingCvesURL: https://security-metadata.canonical.com/oval/com.ubuntu.jammy.cve.oval.xml.bz2
  osFixedCvesURL: https://security-metadata.canonical.com/oval/com.ubuntu.jammy.usn.oval.xml.bz2
  securityFeature: SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION
  platformBundle:
    artifact: edge-orch/edge-node/file/profile-scripts/file/ubuntu-22.04-lts-generic
    artifactVersion: 1.0.16	
`
	EdgeMicrovisorToolkitProfile = `# SPDX-FileCopyrightText: (C) 2024 Intel Corporation
appVersion: apps/v1
metadata:
  release: 3.0.0-dev
  version: 0.2.0
spec:
  name: Edge Microvisor Toolkit 3.0.20250324
  type: OS_TYPE_IMMUTABLE
  provider: OS_PROVIDER_KIND_INFRA
  architecture: x86_64
  profileName: microvisor-nonrt # Name has to be identical to this file's name
  osImageUrl: files-edge-orch/repository/microvisor/non_rt/edge-readonly-3.0.20250324.1008.raw.gz
  osImageVersion: 3.0.20250324.1008
  osImageSha256: 89d691eded21e158e94cf52235106d8eb6c17f81f37b1a79c70514776744bc74
  osPackageManifestURL: files-edge-orch/repository/microvisor/non_rt/edge-readonly-3.0.20250324.1008_pkg_manifest.json
  osExistingCvesURL: files-edge-orch/microvisor/iso/EdgeMicrovisorToolkit-3.1_cve.json
  osFixedCvesURL:
  securityFeature: SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION
  platformBundle:
`
)

var ExampleEdgeMicrovisorToolkitArtifact = as.Artifact{
	Name:      "microvisor-nonrt.yaml",
	MediaType: "application/vnd.oci.image.layer.v1.tar",
	Digest:    "sha256:a2717eccf1539c02000e9329410cb23009191a8c25f47f31e815cb32ac91f6cb",
	Data:      []byte(EdgeMicrovisorToolkitProfile),
}

var ExampleUbuntuOSArtifact = as.Artifact{
	Name:      "ubuntu-22.04-lts-generic.yaml",
	MediaType: "application/vnd.oci.image.layer.v1.tar",
	Digest:    "sha256:8d1a1f184624118cfeee5f31ff6df21ab7a8a4e2262ef66854a1731fa003b5c4",
	Data:      []byte(UbuntuProfile),
}

const defaultTickerPeriodHours = 12

var ExampleOsConfig = common.OsConfig{
	EnabledProfiles:         []string{"ubuntu-22.04-lts-generic"},
	OsProfileRevision:       "main",
	DefaultProfile:          "ubuntu-22.04-lts-generic",
	AutoProvision:           true,
	InventoryTickerPeriod:   defaultTickerPeriodHours * time.Hour,
	OSSecurityFeatureEnable: false,
}

type MockArtifactService struct {
	mock.Mock
}

func (m *MockArtifactService) GetRepositoryTags(_ context.Context, repository string) ([]string, error) {
	args := m.Called(repository)
	repoTags, ok := args.Get(0).([]string)
	if !ok {
		invErr := inv_errors.Errorf("unexpected type for []string: %T", args.Get(0))
		zlog.Err(invErr).Msg("")
		return nil, invErr
	}
	return repoTags, args.Error(1)
}

func (m *MockArtifactService) DownloadArtifacts(_ context.Context, repository, tag string) (*[]as.Artifact, error) {
	args := m.Called(repository, tag)
	artifacts, ok := args.Get(0).(*[]as.Artifact)
	if !ok {
		invErr := inv_errors.Errorf("unexpected type for *[]Artifact: %T", args.Get(0))
		zlog.Err(invErr).Msg("")
		return nil, invErr
	}
	return artifacts, args.Error(1)
}
