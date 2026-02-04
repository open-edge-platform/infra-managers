// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package fsclient

import (
	"compress/bzip2"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	as "github.com/open-edge-platform/infra-core/inventory/v2/pkg/artifactservice"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
)

const (
	semverParts             = 3
	minSemverParts          = 2
	cveCountLimit           = 1000
	ovalDefDescriptionParts = 2
	cveTypeExisting         = "existing"
	cveTypeFixed            = "fixed"
	httpRequestTimeout      = 60 * time.Second // Timeout for HTTP requests (CVE downloads can be large)
)

var (
	zlog = logging.GetLogger("fsclient")
	// EnvNameRsFilesProxyAddress is the environment variable name for RS files proxy address.
	EnvNameRsFilesProxyAddress = "RSPROXY_FILES_ADDRESS"
	// EnvNameRsEnProfileRepo is the environment variable name for RS profile repository.
	EnvNameRsEnProfileRepo = "RS_EN_PROFILE_REPO"
	client                 = &http.Client{
		Timeout: httpRequestTimeout,
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			ForceAttemptHTTP2: false,
		},
	}
)

// OSProfileManifest represents an OS profile manifest.
//
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
		TLSCaCert            string                 `yaml:"tlsCaCertificate"`
	} `yaml:"spec"`
}

// OSPackage represents an OS package.
type OSPackage struct {
	Repo []struct {
		Name    *string `json:"name"`
		Version *string `json:"version"`
	} `json:"repo"`
}

// ExistingCVEs represents existing CVEs.
type ExistingCVEs []struct {
	CveID            *string   `json:"cve_id"`
	Priority         *string   `json:"priority"`
	AffectedPackages []*string `json:"affected_packages"`
}

// FixedCVEs represents fixed CVEs.
type FixedCVEs []struct {
	CveID            *string   `json:"cve_id"`
	Priority         *string   `json:"priority"`
	AffectedPackages []*string `json:"affected_packages"`
}

// OvalDefinitions represents OVAL definitions.
//
// Define the structure based on the OVAL XML format.
type OvalDefinitions struct {
	XMLName     xml.Name     `xml:"oval_definitions"` //nolint:tagliatelle // OVAL XML standard
	Definitions []Definition `xml:"definitions>definition"`
}

// Definition represents an OVAL definition.
type Definition struct {
	ID          string      `xml:"id,attr"`
	Class       string      `xml:"class,attr"`
	Title       string      `xml:"metadata>title"`
	Description string      `xml:"metadata>description"`
	Reference   []Reference `xml:"metadata>reference"`
	Affected    []Affected  `xml:"metadata>affected"`
	Advisory    struct {
		Issued struct {
			Date string `xml:"date,attr"`
		} `xml:"issued"`
	} `xml:"metadata>advisory"`
}

// Reference represents an OVAL reference.
type Reference struct {
	Source string `xml:"source,attr"`
	RefID  string `xml:"ref_id,attr"`  //nolint:tagliatelle // OVAL XML standard
	RefURL string `xml:"ref_url,attr"` //nolint:tagliatelle // OVAL XML standard
}

// Affected represents affected packages.
type Affected struct {
	Family   string `xml:"family,attr"`
	Platform string `xml:"platform"`
}

// CVEInfo represents CVE information.
type CVEInfo struct {
	CVEID            string   `json:"cve_id"`
	Priority         string   `json:"priority"`
	AffectedPackages []string `json:"affected_packages"`
}

func extractPriority(title string) string {
	title = strings.ToLower(title)
	for _, p := range []string{"critical", "high", "medium", "low", "negligible"} {
		if strings.Contains(title, p) {
			return p
		}
	}
	return "unknown"
}

func extractPackages(description string) []string {
	var pkgs []string
	lines := strings.Split(description, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, " - ") && !strings.HasPrefix(line, "Run ") {
			parts := strings.Split(line, " - ")
			if len(parts) == ovalDefDescriptionParts {
				pkg := strings.Fields(parts[0])
				if len(pkg) > 0 {
					pkgs = append(pkgs, pkg[0])
				}
			}
		}
	}
	return pkgs
}

// GetLatestOsProfiles retrieves the latest OS profiles.
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

// GetPackageManifest retrieves the package manifest for an OS profile.
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			zlog.Error().Err(closeErr).Msg("Failed to close response body")
		}
	}()

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

// getCVEsFromURL is a helper function to download and validate CVE data.
func getCVEsFromURL(ctx context.Context, osType, cveURL, cveType string) (string, error) {
	respBody, err := downloadCVEData(ctx, osType, cveURL, cveType)
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
func downloadCVEData(ctx context.Context, osType, cveURL, cveType string) ([]byte, error) {
	switch osType {
	case "OS_TYPE_MUTABLE":
		return downloadMutableCVEData(ctx, cveURL, cveType)
	case "OS_TYPE_IMMUTABLE":
		rsProxyAddress := os.Getenv(EnvNameRsFilesProxyAddress)
		if rsProxyAddress == "" {
			invErr := inv_errors.Errorf("%s env variable is not set", EnvNameRsFilesProxyAddress)
			zlog.Err(invErr).Msg("")
			return nil, invErr
		}
		return downloadImmutableCVEData(ctx, rsProxyAddress, cveURL, cveType)
	default:
		invErr := inv_errors.Errorf("unknown osType: %s", osType)
		zlog.Err(invErr).Msg("Missing URL to download CVE data")
		return nil, invErr
	}
}

// downloadMutableCVEData handles downloading and parsing CVE data for mutable OS types.
func downloadMutableCVEData(ctx context.Context, cveURL, cveType string) ([]byte, error) {
	data, err := downloadAndDecompressCVEData(ctx, cveURL, cveType)
	if err != nil {
		return nil, err
	}

	results, err := parseCVEData(data, cveType)
	if err != nil {
		return nil, err
	}

	return formatCVEResults(results)
}

func downloadAndDecompressCVEData(ctx context.Context, cveURL, cveType string) ([]byte, error) {
	zlog.InfraSec().Info().Msgf("Downloading %s CVEs list from URL: %s", cveType, cveURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cveURL, http.NoBody)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Failed to create GET request to release server: %v", err)
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Failed to connect to release server to download %s CVEs list: %v", cveType, err)
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			zlog.Error().Err(err).Msg("Failed to close response body")
		}
	}()
	bz2Reader := bzip2.NewReader(resp.Body)
	data, readErr := io.ReadAll(bz2Reader)
	if readErr != nil {
		zlog.InfraSec().Error().Err(readErr).
			Msgf("Error reading decompressed data for downloading %s CVEs list: %v", cveType, readErr)
		return nil, readErr
	}
	return data, nil
}

func parseCVEData(data []byte, cveType string) ([]CVEInfo, error) {
	var oval OvalDefinitions
	var unmarshalErr error
	if unmarshalErr = xml.Unmarshal(data, &oval); unmarshalErr != nil {
		zlog.InfraSec().Error().Err(unmarshalErr).
			Msgf("Error parsing XML for downloading %s CVEs list: %v", cveType, unmarshalErr)
		return nil, unmarshalErr
	}
	results := make([]CVEInfo, 0, len(oval.Definitions))
	for _, def := range oval.Definitions {
		cveID, hasUSN := extractCVEIDAndUSN(def, cveType)
		if cveID == "" || hasUSN {
			continue
		}
		priority := extractPriority(def.Title)
		affectedPkgs := extractPackages(def.Description)
		results = append(results, CVEInfo{
			CVEID:            cveID,
			Priority:         priority,
			AffectedPackages: affectedPkgs,
		})
	}
	return results, nil
}

func formatCVEResults(results []CVEInfo) ([]byte, error) {
	if len(results) > cveCountLimit {
		results = results[:cveCountLimit]
	}
	var marshalErr error
	// Convert to the format expected by validateCVEData (with pointers)
	validationResults := make([]struct {
		CveID            *string   `json:"cve_id"`
		Priority         *string   `json:"priority"`
		AffectedPackages []*string `json:"affected_packages"`
	}, 0, len(results))
	for _, result := range results {
		affectedPkgs := make([]*string, 0, len(result.AffectedPackages))
		for _, pkg := range result.AffectedPackages {
			pkgCopy := pkg
			affectedPkgs = append(affectedPkgs, &pkgCopy)
		}
		validationResults = append(validationResults, struct {
			CveID            *string   `json:"cve_id"`
			Priority         *string   `json:"priority"`
			AffectedPackages []*string `json:"affected_packages"`
		}{
			CveID:            &result.CVEID,
			Priority:         &result.Priority,
			AffectedPackages: affectedPkgs,
		})
	}
	respBody, marshalErr := json.MarshalIndent(validationResults, "", "  ")
	if marshalErr != nil {
		zlog.InfraSec().Error().Err(marshalErr).
			Msgf("Error converting to JSON for downloading CVEs list: %v", marshalErr)
		return nil, marshalErr
	}

	return respBody, nil
}

// extractCVEIDAndUSN extracts the CVE ID and USN presence from a definition, reducing cyclomatic complexity.
func extractCVEIDAndUSN(def Definition, cveType string) (string, bool) {
	var cveID string
	hasUSN := false
	for _, ref := range def.Reference {
		if ref.Source == "CVE" {
			cveID = ref.RefID
		}
		if ref.Source == "USN" {
			hasUSN = true
		}
	}
	// For fixed CVEs (USN data), we want to include USN entries, not skip them
	// So we return hasUSN=false to prevent skipping
	if cveType == cveTypeFixed {
		hasUSN = false
	}
	return cveID, hasUSN
}

// downloadImmutableCVEData handles downloading CVE data for immutable OS types.
func downloadImmutableCVEData(ctx context.Context, rsProxyAddress, cveURL, cveType string) ([]byte, error) {
	url := "http://" + rsProxyAddress + cveURL
	zlog.InfraSec().Info().Msgf("Downloading %s CVEs list from URL: %s", cveType, url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Failed to create GET request to release server: %v", err)
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		zlog.InfraSec().Error().Err(err).Msgf("Failed to connect to release server to download %s CVEs list: %v", cveType, err)
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			zlog.Error().Err(closeErr).Msg("Failed to close response body")
		}
	}()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		zlog.InfraSec().Error().Err(err).
			Msgf("Failed to read the %s CVEs list content: %v", cveType, err)
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
		zlog.InfraSec().Error().Err(err).Msgf("Invalid %s CVEs list content returned from Release Service: %v", cveType, err)
		return err
	}

	// validate OS CVEs list content
	if len(cves) == 0 || cves[0].CveID == nil || cves[0].Priority == nil {
		invErr := inv_errors.Errorf("missing mandatory fields in %s CVEs list content returned from Release Service", cveType)
		zlog.InfraSec().Error().Err(invErr).Msgf("OS %s CVEs list sanity failed", cveType)
		return invErr
	}

	return nil
}

// GetExistingCVEs retrieves existing CVEs for an OS profile.
func GetExistingCVEs(ctx context.Context, osType, existingCVEsURL string) (string, error) {
	return getCVEsFromURL(ctx, osType, existingCVEsURL, cveTypeExisting)
}

// GetFixedCVEs retrieves fixed CVEs for an OS profile.
func GetFixedCVEs(ctx context.Context, osType, fixedCVEsURL string) (string, error) {
	return getCVEsFromURL(ctx, osType, fixedCVEsURL, cveTypeFixed)
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
