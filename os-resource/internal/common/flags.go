// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0
package common

import "flag"

const (
	ManualMode                         = "manualMode"
	ManualModeDescription              = "Flag to enable manual mode"
	OsProfileRevision                  = "osProfileRevision"
	OsProfileRevisionDescription       = "Specifies the release version of OS profiles in the Release Service"
	EnabledProfiles                    = "enabledProfiles"
	EnabledProfilesDescription         = "Specifies a list of profile names"
	DefaultProfile                     = "defaultProfile"
	DefaultProfileDescription          = "Specifies a single profile name that is used by default for ZTP"
	AutoprovisionFlag                  = "autoProvisionEnabled"
	AutoprovisionDescription           = "Specifies whether to configure ZTP mode"
	OSSecurityFeatureEnable            = "osSecurityFeatureEnable"
	OSSecurityFeatureEnableDescription = "Specifies whether to enable or disable security feature"
)

var DisableProviderAutomationFlag = flag.Bool("disableProviderAutomation", false,
	"If set to true, OSRM doesn't auto-create infra_onboarding Provider")
