// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"flag"
	"testing"

	"github.com/open-edge-platform/infra-managers/remote-access/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Unused in these tests
	flag.String("globalLogLevel", "debug", "log level")
	flag.Parse()
	m.Run()
}

func Test_FormatTenantResourceID(t *testing.T) {
	assert.Equal(t, "[tenantID=, resourceID=]", utils.FormatTenantResourceID("", ""))
	assert.Equal(t, "[tenantID=tID, resourceID=rID]", utils.FormatTenantResourceID("tID", "rID"))
}
