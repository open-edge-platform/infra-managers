// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"google.golang.org/grpc/codes"

	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
)

type OsConfig struct {
	EnabledProfiles         []string
	OsProfileRevision       string
	DefaultProfile          string
	AutoProvision           bool
	ManualMode              bool
	OSSecurityFeatureEnable bool
}

func (c OsConfig) validateDefaultProfile() error {
	foundDefaultProfile := false
	for _, pName := range c.EnabledProfiles {
		if pName == c.DefaultProfile {
			foundDefaultProfile = true
		}
	}

	if !foundDefaultProfile {
		return inv_errors.Errorfc(codes.InvalidArgument,
			"Default profile '%s' is not included in enabled profiles [%s]",
			c.DefaultProfile, c.EnabledProfiles)
	}

	return nil
}

func (c OsConfig) Validate() error {
	if c.OsProfileRevision == "" || len(c.EnabledProfiles) == 0 {
		return inv_errors.Errorfc(codes.InvalidArgument, "Mandatory config values are not provided: %+v", c)
	}

	if c.AutoProvision && c.DefaultProfile == "" {
		return inv_errors.Errorfc(codes.InvalidArgument, "ZTP enabled but no default profile provided: %+v", c)
	}

	if c.AutoProvision {
		if err := c.validateDefaultProfile(); err != nil {
			return err
		}
	}

	return nil
}
