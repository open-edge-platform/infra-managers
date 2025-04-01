// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package statusdetail_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	sd "github.com/open-edge-platform/infra-managers/maintenance/pkg/statusdetail"
)

func TestUnmarshalJSON_UpdateType(t *testing.T) {
	var ut sd.UpdateType

	validUpTypeJSON := map[string]sd.UpdateType{
		`"OS"`:            sd.OS,
		`"application"`:   sd.Application,
		`"firmware"`:      sd.Firmware,
		`"container"`:     sd.Container,
		`"configuration"`: sd.Configuration,
	}

	invalidUpTypeJSON := map[string]sd.UpdateType{
		`"system"`:  sd.OS,
		`"app"`:     sd.Application,
		`"fw"`:      sd.Firmware,
		`"contain"`: sd.Container,
		`"config"`:  sd.Configuration,
	}

	tests := []struct {
		name    string
		upType  map[string]sd.UpdateType
		errWant bool
	}{
		{
			name:    "ReturnValidStatusType",
			upType:  validUpTypeJSON,
			errWant: false,
		},
		{
			name:    "ReturnInvalidStatusType",
			upType:  invalidUpTypeJSON,
			errWant: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for jsonStr, expected := range tt.upType {
				err := json.Unmarshal([]byte(jsonStr), &ut)

				if tt.errWant {
					assert.NotNil(t, err)
				} else {
					assert.Equal(t, expected, ut)
				}
			}
		})
	}
}

func TestUnmarshalJSON_Action(t *testing.T) {
	validActionJSON := map[string]sd.Action{
		`"upgrade"`: sd.Upgrade,
		`"install"`: sd.Install,
	}

	invalidActionJSON := map[string]sd.Action{
		`"grade"`:        sd.Upgrade,
		`"installation"`: sd.Install,
	}

	var a sd.Action

	tests := []struct {
		name    string
		action  map[string]sd.Action
		errWant bool
	}{
		{
			name:    "ReturnValidAction",
			action:  validActionJSON,
			errWant: false,
		},
		{
			name:    "ReturnInvalidAction",
			action:  invalidActionJSON,
			errWant: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for jsonStr, expected := range tt.action {
				err := json.Unmarshal([]byte(jsonStr), &a)

				if tt.errWant {
					assert.NotNil(t, err)
				} else {
					assert.Equal(t, expected, a)
				}
			}
		})
	}
}

func TestUnmarshalJSON_FailureReason(t *testing.T) {
	validFailureReasonJSON := map[string]sd.FailureReason{
		`"unspecified"`:         sd.Unspecified,
		`"nofailure"`:           sd.NoFailure,
		`"download"`:            sd.Download,
		`"bootloader"`:          sd.Bootloader,
		`"insufficientstorage"`: sd.InsufficientStorage,
		`"rsauthentication"`:    sd.RSAuthentication,
		`"signaturecheck"`:      sd.SignatureCheck,
		`"utwrite"`:             sd.UTWrite,
		`"utbootconfiguration"`: sd.UTBootConfiguration,
		`"criticalservices"`:    sd.CriticalServices,
		`"inbm"`:                sd.INBM,
		`"oscommit"`:            sd.OSCommit,
	}

	invalidFailureReasonJSON := map[string]sd.FailureReason{
		`"unknown"`:            sd.Unspecified,
		`"none"`:               sd.NoFailure,
		`"downloa"`:            sd.Download,
		`"botloader"`:          sd.Bootloader,
		`"nsufficientstorage"`: sd.InsufficientStorage,
		`"authentication"`:     sd.RSAuthentication,
		`"signature"`:          sd.SignatureCheck,
		`"write"`:              sd.UTWrite,
		`"tbootconfiguration"`: sd.UTBootConfiguration,
		`"riticalservices"`:    sd.CriticalServices,
		`"nbm"`:                sd.INBM,
		`"scommit"`:            sd.OSCommit,
	}

	var fr sd.FailureReason

	tests := []struct {
		name    string
		reason  map[string]sd.FailureReason
		errWant bool
	}{
		{
			name:    "ReturnValidAction",
			reason:  validFailureReasonJSON,
			errWant: false,
		},
		{
			name:    "ReturnInvalidAction",
			reason:  invalidFailureReasonJSON,
			errWant: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for jsonStr, expected := range tt.reason {
				err := json.Unmarshal([]byte(jsonStr), &fr)

				if tt.errWant {
					assert.NotNil(t, err)
				} else {
					assert.Equal(t, expected, fr)
				}
			}
		})
	}
}

func TestUnmarshalJSON_Status(t *testing.T) {
	var s sd.Status

	validStatusJSON := map[string]sd.Status{
		`"installed"`:  sd.Installed,
		`"failed"`:     sd.Failed,
		`"rolledback"`: sd.Rolledback,
	}

	invalidStatusJSON := map[string]sd.Status{
		`"install"`:  sd.Installed,
		`"fail"`:     sd.Failed,
		`"rollback"`: sd.Rolledback,
	}

	tests := []struct {
		name    string
		status  map[string]sd.Status
		errWant bool
	}{
		{
			name:    "ReturnValidStatus",
			status:  validStatusJSON,
			errWant: false,
		},
		{
			name:    "ReturnInvalidStatus",
			status:  invalidStatusJSON,
			errWant: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for jsonStr, expected := range tt.status {
				err := json.Unmarshal([]byte(jsonStr), &s)

				if tt.errWant {
					assert.NotNil(t, err)
				} else {
					assert.Equal(t, expected, s)
				}
			}
		})
	}
}
