// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

//nolint:testpackage // testing internal functions
package fsclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	as "github.com/open-edge-platform/infra-core/inventory/v2/pkg/artifactservice"
	osrm_testing "github.com/open-edge-platform/infra-managers/os-resource/internal/testing"
)

const (
	OSPackageManifest = `{
"Repo": [
  {
    "Name": "zlib",
    "Version": "1.3.1-1",
    "Architecture": "x86_64",
    "Distribution": "tmv3",
    "URL": "https://www.zlib.net/",
    "License": "zlib",
    "Modified": "No"
  },
  {
    "Name": "openssl-libs",
    "Version": "3.3.2-1",
    "Architecture": "x86_64",
    "Distribution": "tmv3",
    "URL": "http://www.openssl.org/",
    "License": "Apache-2.0",
    "Modified": "No"
  },
  {
    "Name": "initramfs",
    "Version": "3.0-5",
    "Architecture": "x86_64",
    "Distribution": "tmv3",
    "URL": "(null)",
    "License": "Apache License",
    "Modified": "No"
  }
]
}
`
	InvalidOSPackageManifest = `{
"Repo": [
  {
    "Architecture": "x86_64",
    "Distribution": "tmv3",
    "URL": "http://www.openssl.org/",
    "License": "Apache-2.0",
    "Modified": "No"
  },
  {
    "Architecture": "x86_64",
    "Distribution": "tmv3",
    "URL": "(null)",
    "License": "Apache License",
    "Modified": "No"
  }
]
}
`

	ExistingCVEsList = `[
  {
    "cve_id": "CVE-2024-1234",
    "priority": "HIGH",
    "affected_packages": ["openssl", "libssl1.1"]
  },
  {
    "cve_id": "CVE-2024-5678",
    "priority": "MEDIUM",
    "affected_packages": ["zlib1g", "zlib1g-dev"]
  },
  {
    "cve_id": "CVE-2024-9999",
    "priority": "LOW",
    "affected_packages": ["curl", "libcurl4"]
  }
]`

	FixedCVEsList = `[
  {
    "cve_id": "CVE-2023-1111",
    "priority": "CRITICAL",
    "affected_packages": ["kernel", "linux-headers"]
  },
  {
    "cve_id": "CVE-2023-2222",
    "priority": "HIGH",
    "affected_packages": ["nginx", "nginx-common"]
  }
]`

	InvalidExistingCVEsList = `[
  {
    "priority": "HIGH",
    "affected_packages": ["openssl", "libssl1.1"]
  }
]`

	InvalidFixedCVEsList = `[
  {
    "cve_id": "CVE-2023-1111",
    "affected_packages": ["kernel", "linux-headers"]
  }
]`

	EmptyCVEsList = `[]`
)

func Test_GetLatestOsProfiles(t *testing.T) {
	m := &osrm_testing.MockArtifactService{}
	as.DefaultArtService = m

	var ubuntuProfile OSProfileManifest
	if err := yaml.Unmarshal(osrm_testing.ExampleUbuntuOSArtifact.Data, &ubuntuProfile); err != nil {
		t.Errorf("error unmarshalling ExampleUbuntuOSArtifact.Data JSON")
	}

	var edgeMicrovisorToolkitProfile OSProfileManifest
	if err := yaml.Unmarshal(osrm_testing.ExampleEdgeMicrovisorToolkitArtifact.Data, &edgeMicrovisorToolkitProfile); err != nil {
		t.Errorf("error unmarshalling ExampleEdgeMicrovisorToolkitArtifact.Data JSON")
	}

	type args struct {
		ctx          context.Context
		profileNames []string
		tag          string
		artifacts    []as.Artifact
	}
	tests := []struct {
		name              string
		args              args
		downloadOSProfile error
		wantErr           bool
	}{
		{
			name: "successful - return two OS profiles",
			args: args{
				ctx:          context.Background(),
				profileNames: []string{ubuntuProfile.Spec.ProfileName, edgeMicrovisorToolkitProfile.Spec.ProfileName},
				tag:          "24.11.0",
				artifacts: []as.Artifact{
					osrm_testing.ExampleUbuntuOSArtifact,
					osrm_testing.ExampleEdgeMicrovisorToolkitArtifact,
				},
			},
			downloadOSProfile: nil,
			wantErr:           false,
		},
		{
			name: "successful - empty OS profiles request",
			args: args{
				ctx:          context.Background(),
				profileNames: []string{},
				tag:          "24.11.0",
				artifacts:    nil,
			},
			downloadOSProfile: nil,
			wantErr:           false,
		},
		{
			name: "failure - missing tag",
			args: args{
				ctx:          context.Background(),
				profileNames: []string{ubuntuProfile.Spec.ProfileName},
				tag:          "",
				artifacts:    []as.Artifact{osrm_testing.ExampleUbuntuOSArtifact},
			},
			downloadOSProfile: fmt.Errorf("tag is missing"),
			wantErr:           true,
		},
	}

	// set RS_EN_PROFILE_REPO env variable needed by GetLatestOsProfiles()
	t.Setenv(EnvNameRsEnProfileRepo, osrm_testing.EnProfileRepo)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for idx := range tt.args.profileNames {
				m.On("DownloadArtifacts", osrm_testing.EnProfileRepo+tt.args.profileNames[idx], tt.args.tag).
					Return(&[]as.Artifact{tt.args.artifacts[idx]}, tt.downloadOSProfile)
			}

			osProfiles, err := GetLatestOsProfiles(tt.args.ctx, tt.args.profileNames, tt.args.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLatestOsProfiles() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				assert.Len(t, osProfiles, len(tt.args.profileNames))
				for _, name := range tt.args.profileNames {
					profileSlice := osProfiles[name]
					assert.NotEmpty(t, profileSlice)
					for _, profile := range profileSlice {
						assert.Equal(t, name, profile.Spec.ProfileName)
					}
				}
			}
		})
	}
}

func Test_GetPackageManifest(t *testing.T) {
	mux := http.NewServeMux()

	type args struct {
		url             string
		packageManifest string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Successful - valid manifest content",
			args: args{
				url:             "/validmanifest",
				packageManifest: OSPackageManifest,
			},
			wantErr: false,
		},
		{
			name: "Failure - non-JSON manifest content",
			args: args{
				url:             "/nonjsonmanifest",
				packageManifest: "Non-JSON content!",
			},
			wantErr: true,
		},
		{
			name: "Failure - empty manifest content",
			args: args{
				url:             "/emptymanifest",
				packageManifest: "",
			},
			wantErr: true,
		},
		{
			name: "Failure - incorrect JSON content",
			args: args{
				url:             "/incorrectjsoncontent",
				packageManifest: InvalidOSPackageManifest,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		// serve OS package manifest in httptest
		mux.HandleFunc(tt.args.url, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(tt.args.packageManifest)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		})
	}

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	// replace rs-proxy URL with the httptest local server address
	t.Setenv(EnvNameRsFilesProxyAddress, strings.TrimPrefix(httpServer.URL, "http://"))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packageManifest, err := GetPackageManifest(context.Background(), tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPackageManifest() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				assert.NoError(t, err)
				assert.NotEmpty(t, packageManifest)
			}
		})
	}
}

func Test_GetCVEs_Success(t *testing.T) {
	mux := http.NewServeMux()

	type args struct {
		url      string
		cvesList string
	}

	type testCase struct {
		name    string
		args    args
		cveType string // "existing" or "fixed"
	}

	tests := []testCase{
		{
			name: "GetExistingCVEs - Successful with valid existing CVEs list",
			args: args{
				url:      "/validexistingcves",
				cvesList: ExistingCVEsList,
			},
			cveType: "existing",
		},
		{
			name: "GetFixedCVEs - Successful with valid fixed CVEs list",
			args: args{
				url:      "/validfixedcves",
				cvesList: FixedCVEsList,
			},
			cveType: "fixed",
		},
	}

	for _, tt := range tests {
		// serve CVEs list in httptest
		mux.HandleFunc(tt.args.url, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(tt.args.cvesList)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		})
	}

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	// replace rs-proxy URL with the httptest local server address
	t.Setenv(EnvNameRsFilesProxyAddress, strings.TrimPrefix(httpServer.URL, "http://"))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cves string
			var err error

			if tt.cveType == "existing" {
				cves, err = GetExistingCVEs(context.Background(), "OS_TYPE_IMMUTABLE", tt.args.url)
			} else {
				cves, err = GetFixedCVEs(context.Background(), "OS_TYPE_IMMUTABLE", tt.args.url)
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, cves)
			// Verify the response doesn't contain spaces or newlines
			assert.NotContains(t, cves, " ")
			assert.NotContains(t, cves, "\n")
		})
	}
}

func Test_GetCVEs_Failure(t *testing.T) {
	mux := http.NewServeMux()

	type args struct {
		url      string
		cvesList string
	}

	type testCase struct {
		name    string
		args    args
		cveType string // "existing" or "fixed"
	}

	tests := []testCase{
		{
			name: "GetExistingCVEs - Failure with non-JSON CVEs content",
			args: args{
				url:      "/nonjsonexistingcves",
				cvesList: "Non-JSON content!",
			},
			cveType: "existing",
		},
		{
			name: "GetExistingCVEs - Failure with empty CVEs list",
			args: args{
				url:      "/emptyexistingcves",
				cvesList: EmptyCVEsList,
			},
			cveType: "existing",
		},
		{
			name: "GetExistingCVEs - Failure with completely empty response",
			args: args{
				url:      "/emptyexistingresponse",
				cvesList: "",
			},
			cveType: "existing",
		},
		{
			name: "GetFixedCVEs - Failure with non-JSON CVEs content",
			args: args{
				url:      "/nonjsonfixedcves",
				cvesList: "Non-JSON content!",
			},
			cveType: "fixed",
		},
		{
			name: "GetFixedCVEs - Failure with completely empty response",
			args: args{
				url:      "/emptyfixedresponse",
				cvesList: "",
			},
			cveType: "fixed",
		},
	}

	for _, tt := range tests {
		// serve CVEs list in httptest
		mux.HandleFunc(tt.args.url, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(tt.args.cvesList)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		})
	}

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	// replace rs-proxy URL with the httptest local server address
	t.Setenv(EnvNameRsFilesProxyAddress, strings.TrimPrefix(httpServer.URL, "http://"))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error

			if tt.cveType == "existing" {
				_, err = GetExistingCVEs(context.Background(), "OS_TYPE_IMMUTABLE", tt.args.url)
			} else {
				_, err = GetFixedCVEs(context.Background(), "OS_TYPE_IMMUTABLE", tt.args.url)
			}

			assert.Error(t, err)
		})
	}
}

func Test_GetCVEs_MissingEnvVar(t *testing.T) {
	// Unset the environment variable to test error handling
	t.Setenv(EnvNameRsFilesProxyAddress, "")

	_, err := GetExistingCVEs(context.Background(), "OS_TYPE_IMMUTABLE", "/somepath")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "env variable is not set")

	_, err = GetFixedCVEs(context.Background(), "OS_TYPE_IMMUTABLE", "/somepath")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "env variable is not set")
}
