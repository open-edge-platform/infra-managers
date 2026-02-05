// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package common provides common configuration and flags.
//
//nolint:revive // Package name 'common' is intentional for shared utilities
package common

import "flag"

const (
	// OsProfileRevision is the OS profile revision flag name.
	OsProfileRevision = "osProfileRevision"
	// OsProfileRevisionDescription describes the OsProfileRevision flag.
	OsProfileRevisionDescription = "Specifies the release version of OS profiles in the Release Service"
	// EnabledProfiles is the enabled profiles flag name.
	EnabledProfiles                    = "enabledProfiles"
	EnabledProfilesDescription         = "Specifies a list of profile names"
	DefaultProfile                     = "defaultProfile"
	DefaultProfileDescription          = "Specifies a single profile name that is used by default for ZTP"
	AutoprovisionFlag                  = "autoProvisionEnabled"
	AutoprovisionDescription           = "Specifies whether to configure ZTP mode"
	OSSecurityFeatureEnable            = "osSecurityFeatureEnable"
	OSSecurityFeatureEnableDescription = "Specifies whether to enable or disable security feature"
)

// DisableProviderAutomationFlag disables provider automation.
var DisableProviderAutomationFlag = flag.Bool("disableProviderAutomation", false,
	"If set to true, OSRM doesn't auto-create infra_onboarding Provider")
