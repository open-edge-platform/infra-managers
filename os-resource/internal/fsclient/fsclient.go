// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package fsclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	as "github.com/open-edge-platform/infra-core/inventory/v2/pkg/artifactservice"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
)

var (
	zlog                       = logging.GetLogger("fsclient")
	EnvNameRsFilesProxyAddress = "RSPROXY_FILES_ADDRESS"
	EnvNameRsEnProfileRepo     = "RS_EN_PROFILE_REPO"
	client                     = &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			ForceAttemptHTTP2: false,
		},
	}
)

// Maintain consistency with OS profiles
//
//nolint:tagliatelle // Renaming the yaml keys may effect while unmarshalling/marshaling so, used nolint.
type OSProfileManifest struct {
	AppVersion string                 `yaml:"appVersion"`
	Metadata   map[string]interface{} `yaml:"metadata"`
	Spec       struct {
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
	} `yaml:"spec"`
}

type OSPackage struct {
	Repo []struct {
		Name    *string `json:"name"`
		Version *string `json:"version"`
	} `json:"repo"`
}

func GetLatestOsProfiles(ctx context.Context, profileNames []string, tag string) (map[string]*OSProfileManifest, error) {
	enProfileRepo := os.Getenv(EnvNameRsEnProfileRepo)
	if enProfileRepo == "" {
		invErr := inv_errors.Errorf("%s env variable is not set", EnvNameRsEnProfileRepo)
		zlog.Err(invErr).Msg("")
		return map[string]*OSProfileManifest{}, invErr
	}

	osProfiles := make(map[string]*OSProfileManifest)
	for _, pName := range profileNames {
		artifacts, err := as.DownloadArtifacts(ctx, enProfileRepo+pName, tag)
		if err != nil || artifacts == nil || len(*artifacts) == 0 {
			invErr := inv_errors.Errorf("Error downloading OS profile manifest for profile name %s and tag %s from Repo: %s",
				pName, tag, enProfileRepo+pName)
			zlog.InfraSec().Error().Err(invErr).Msg(err.Error())
			return map[string]*OSProfileManifest{}, invErr
		}

		var enManifest OSProfileManifest
		if err := yaml.Unmarshal((*artifacts)[0].Data, &enManifest); err != nil {
			zlog.InfraSec().Error().Err(err).Msg("Error unmarshalling OSProfileManifest JSON")
			return map[string]*OSProfileManifest{}, inv_errors.Wrap(err)
		}
		osProfiles[pName] = &enManifest
	}
	return osProfiles, nil
}

func GetPackageManifest(ctx context.Context, packageManifestURL string) (string, error) {
	rsProxyAddress := os.Getenv(EnvNameRsFilesProxyAddress)
	if rsProxyAddress == "" {
		invErr := inv_errors.Errorf("%s env variable is not set", EnvNameRsFilesProxyAddress)
		zlog.Err(invErr).Msg("")
		return "", invErr
	}

	url := "http://" + rsProxyAddress + packageManifestURL
	zlog.InfraSec().Info().Msgf("Downloading package manifest from URL: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Failed to create GET request to release server: %v", err)
		return "", err
	}

	// Perform the HTTP GET request
	resp, err := client.Do(req)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Failed to connect to release server to download package manifest: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Failed to read the package manifest content: %v", err)
		return "", err
	}

	// verify that the returned response from RS is valid JSON
	var osPackages OSPackage
	if err := json.Unmarshal(respBody, &osPackages); err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Invalid package manifest content returned from Release Service: %v", err)
		return "", err
	}

	// validate OS package manifest content
	if len(osPackages.Repo) == 0 || osPackages.Repo[0].Name == nil || osPackages.Repo[0].Version == nil {
		invErr := inv_errors.Errorf("missing mandatory fields in package manifest content returned from Release Service")
		zlog.InfraSec().Error().Err(invErr).Msgf("OS Package manifest sanity failed")
		return "", invErr
	}

	// strip all white spaces from JSON content
	packageManifest := string(respBody)
	packageManifest = strings.ReplaceAll(packageManifest, "\n", "")
	packageManifest = strings.ReplaceAll(packageManifest, " ", "")
	return packageManifest, nil
}
