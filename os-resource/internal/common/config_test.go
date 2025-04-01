// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package common_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	osrm_testing "github.com/open-edge-platform/infra-managers/os-resource/internal/testing"
)

func TestValidate(t *testing.T) {
	osConfig := osrm_testing.ExampleOsConfig
	err := osConfig.Validate()
	assert.NoError(t, err)
}

func TestValidateInvalidProfile(t *testing.T) {
	osConfig := osrm_testing.ExampleOsConfig
	osConfig.DefaultProfile = "invalid-profile"
	err := osConfig.Validate()
	assert.Error(t, err)
}
