// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package fsclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"

	as "github.com/open-edge-platform/infra-core/inventory/v2/pkg/artifactservice"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
)

const (
	semverParts    = 3
	minSemverParts = 2
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
	AppVersion string            `yaml:"appVersion"`
	Metadata   map[string]string `yaml:"metadata"`
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
		OsExistingCvesURL    string                 `yaml:"osExistingCvesURL"`
		OsFixedCvesURL       string                 `yaml:"osFixedCvesURL"`
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

type ExistingCVEs []struct {
	CveID            *string   `json:"cve_id"`
	Priority         *string   `json:"priority"`
	AffectedPackages []*string `json:"affected_packages"`
}

type FixedCVEs []struct {
	CveID            *string   `json:"cve_id"`
	Priority         *string   `json:"priority"`
	AffectedPackages []*string `json:"affected_packages"`
}

func GetLatestOsProfiles(ctx context.Context, profileNames []string, tag string) (map[string][]*OSProfileManifest, error) {
	enProfileRepo, err := getProfileRepoFromEnv()
	if err != nil {
		return map[string][]*OSProfileManifest{}, err
	}

	// For each of the enabled profiles,
	//  - fetch all the revisions if tag contains a semver range (tag with ~ as prefix) else fetch the specific revision as in tag
	//  - fetch the os profile for each of the revisions of that enabled profile and store in a golang map
	osProfiles := make(map[string][]*OSProfileManifest)
	for _, pName := range profileNames {
		zlog.InfraSec().Info().Msgf("OS profile: %s:%s", pName, tag)
		osProfileRevisions, err := getProfileRevisionsFromProfileRepo(ctx, enProfileRepo, pName, tag)
		if err != nil {
			return map[string][]*OSProfileManifest{}, err
		}
		for _, osProfileRevision := range osProfileRevisions {
			zlog.InfraSec().Info().Msgf("Fetching OS profile for %s:%s", pName, osProfileRevision)
			manifest, err := fetchOSProfile(ctx, enProfileRepo, pName, osProfileRevision)
			if err != nil {
				return map[string][]*OSProfileManifest{}, err
			}
			osProfiles[pName] = append(osProfiles[pName], manifest)
		}
	}
	return osProfiles, nil
}

func getProfileRepoFromEnv() (string, error) {
	enProfileRepo := os.Getenv(EnvNameRsEnProfileRepo)
	if enProfileRepo == "" {
		invErr := inv_errors.Errorf("%s env variable is not set", EnvNameRsEnProfileRepo)
		zlog.Err(invErr).Msg("")
		return "", invErr
	}
	return enProfileRepo, nil
}

func getProfileRevisionsFromProfileRepo(ctx context.Context, enProfileRepo, pName, tag string) ([]string, error) {
	var osProfileRevisions []string
	if strings.HasPrefix(tag, "~") {
		profileRevisionStart := strings.TrimPrefix(tag, "~")
		major, minor, patch := ParseSemver(profileRevisionStart)

		pNameRevisions, err := as.GetRepositoryTags(ctx, enProfileRepo+pName)
		if err != nil || len(pNameRevisions) == 0 {
			invErr := inv_errors.Errorf("Error getting os profile revisions for profile name %s from Repo: %s",
				pName, enProfileRepo+pName)
			zlog.InfraSec().Error().Err(invErr).Msg(err.Error())
			return nil, invErr
		}

		for _, pNameRevision := range pNameRevisions {
			pNameRevisionMajor, pNameRevisionMinor, pNameRevisionPatch := ParseSemver(pNameRevision)
			if pNameRevisionMajor == major && pNameRevisionMinor == minor && pNameRevisionPatch >= patch {
				osProfileRevisions = append(osProfileRevisions, pNameRevision)
			}
		}
	} else {
		osProfileRevisions = append(osProfileRevisions, tag)
	}
	return osProfileRevisions, nil
}

func fetchOSProfile(ctx context.Context, repo, profileName, tag string) (*OSProfileManifest, error) {
	artifacts, err := as.DownloadArtifacts(ctx, repo+profileName, tag)
	if err != nil || artifacts == nil || len(*artifacts) == 0 {
		invErr := inv_errors.Errorf(
			"Error downloading OS profile manifest for profile name %s and osProfileRevision %s from Repo: %s",
			profileName, tag, repo+profileName)
		zlog.InfraSec().Error().Err(invErr).Msg(err.Error())
		return nil, invErr
	}
	var enManifest OSProfileManifest
	if err := yaml.Unmarshal((*artifacts)[0].Data, &enManifest); err != nil {
		zlog.InfraSec().Error().Err(err).Msg("Error unmarshalling OSProfileManifest JSON")
		return nil, inv_errors.Wrap(err)
	}

	return &enManifest, nil
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
		zlog.InfraSec().Error().Err(err).Msg("Failed to create GET request to release server")
		return "", err
	}

	// Perform the HTTP GET request
	resp, err := client.Do(req)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msg("Failed to connect to release server to download package manifest")
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msg("Failed to read the package manifest content")
		return "", err
	}

	// verify that the returned response from RS is valid JSON
	var osPackages OSPackage
	if err := json.Unmarshal(respBody, &osPackages); err != nil {
		zlog.InfraSec().Error().Err(err).Msg("Invalid package manifest content returned from Release Service")
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

// getCVEsFromURL is a helper function to download and validate CVE data.
func getCVEsFromURL(ctx context.Context, cveURL, cveType string) (string, error) {
	rsProxyAddress := os.Getenv(EnvNameRsFilesProxyAddress)
	if rsProxyAddress == "" {
		invErr := inv_errors.Errorf("%s env variable is not set", EnvNameRsFilesProxyAddress)
		zlog.Err(invErr).Msg("")
		return "", invErr
	}

	respBody, err := downloadCVEData(ctx, rsProxyAddress, cveURL, cveType)
	if err != nil {
		return "", err
	}

	if err := validateCVEData(respBody, cveType); err != nil {
		return "", err
	}

	// strip all white spaces from JSON content
	cvesList := string(respBody)
	cvesList = strings.ReplaceAll(cvesList, "\n", "")
	cvesList = strings.ReplaceAll(cvesList, " ", "")
	return cvesList, nil
}

// downloadCVEData downloads CVE data from the given URL.
func downloadCVEData(ctx context.Context, rsProxyAddress, cveURL, cveType string) ([]byte, error) {
	url := "http://" + rsProxyAddress + cveURL
	zlog.InfraSec().Info().Msgf("Downloading %s CVEs list from URL: %s", cveType, url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msg("Failed to create GET request to release server")
		return nil, err
	}

	// Perform the HTTP GET request
	resp, err := client.Do(req)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Failed to connect to release server to download %s CVEs list", cveType)
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Failed to read the %s CVEs list content", cveType)
		return nil, err
	}

	return respBody, nil
}

// validateCVEData validates the structure and content of CVE data.
func validateCVEData(respBody []byte, cveType string) error {
	// verify that the returned response from RS is valid JSON
	var cves []struct {
		CveID            *string   `json:"cve_id"`
		Priority         *string   `json:"priority"`
		AffectedPackages []*string `json:"affected_packages"`
	}
	if err := json.Unmarshal(respBody, &cves); err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Invalid %s CVEs list content returned from Release Service", cveType)
		return err
	}

	// validate OS CVEs list content
	if len(cves) == 0 || cves[0].CveID == nil || cves[0].Priority == nil ||
		len(cves[0].AffectedPackages) == 0 || cves[0].AffectedPackages[0] == nil {
		invErr := inv_errors.Errorf("missing mandatory fields in %s CVEs list content returned from Release Service", cveType)
		zlog.InfraSec().Error().Err(invErr).Msgf("OS %s CVEs list sanity failed", cveType)
		return invErr
	}

	return nil
}

func GetExistingCVEs(ctx context.Context, existingCVEsURL string) (string, error) {
	return getCVEsFromURL(ctx, existingCVEsURL, "existing")
}

func GetFixedCVEs(ctx context.Context, fixedCVEsURL string) (string, error) {
	return getCVEsFromURL(ctx, fixedCVEsURL, "fixed")
}

// ParseSemver parses a semver version string (e.g., "1.2.3") and returns major, minor, and patch as integers.
// If any part is missing or invalid, it defaults to 0 for that part.
func ParseSemver(version string) (int, int, int) {
	parts := strings.SplitN(version, ".", semverParts)
	var major, minor, patch int
	var err error
	if len(parts) >= 1 {
		major, err = strconv.Atoi(parts[0])
		if err != nil {
			major = 0
		}
	}
	if len(parts) >= minSemverParts {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			minor = 0
		}
	}
	if len(parts) == semverParts {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			patch = 0
		}
	}
	return major, minor, patch
}
