// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package statusdetail provides status detail types for OS updates.
package statusdetail

import (
	"encoding/json"
	"fmt"
	"time"
)

type (
	// Status represents the status of an update operation.
	Status int
	// Action represents the action type of an update.
	Action int
	// UpdateType represents the type of update being performed.
	UpdateType int
	// FailureReason represents the reason for update failure.
	FailureReason int
)

const (
	// Installed represents an installed status.
	Installed Status = iota
	// Failed represents a failed status.
	Failed
	// Rolledback represents a rolled back status.
	Rolledback
)

const (
	// Upgrade represents an upgrade action.
	Upgrade Action = iota
	// Install represents an install action.
	Install
)

const (
	// OS represents an OS update type.
	OS UpdateType = iota
	// Application represents an application update type.
	Application
	// Firmware represents a firmware update type.
	Firmware
	// Container represents a container update type.
	Container
	// Configuration represents a configuration update type.
	Configuration
)

const (
	// Unspecified represents an unspecified failure reason.
	Unspecified FailureReason = iota
	// NoFailure represents no failure.
	NoFailure
	// Download represents a download failure.
	Download
	// Installation represents an installation failure.
	Installation
	// Bootloader represents a bootloader failure.
	Bootloader
	// InsufficientStorage represents an insufficient storage failure.
	InsufficientStorage
	// RSAuthentication represents a remote storage authentication failure.
	RSAuthentication
	// SignatureCheck represents a signature check failure.
	SignatureCheck
	// UTWrite represents a unified threat write failure.
	UTWrite
	// UTBootConfiguration represents a unified threat boot configuration failure.
	UTBootConfiguration
	// CriticalServices represents a critical services failure.
	CriticalServices
	// INBM represents an INBM failure.
	INBM
	// OSCommit represents an OS commit failure.
	OSCommit
)

var stringToUpdateType = map[string]UpdateType{
	"OS":            OS,
	"application":   Application,
	"firmware":      Firmware,
	"container":     Container,
	"configuration": Configuration,
}

var stringToAction = map[string]Action{
	"upgrade": Upgrade,
	"install": Install,
}

var stringToStatus = map[string]Status{
	"installed":  Installed,
	"failed":     Failed,
	"rolledback": Rolledback,
}

// must be kept in sync with failure reasons returned by PUA.
var stringToFailureReason = map[string]FailureReason{
	"unspecified":         Unspecified,
	"nofailure":           NoFailure,
	"download":            Download,
	"bootloader":          Bootloader,
	"insufficientstorage": InsufficientStorage,
	"rsauthentication":    RSAuthentication,
	"signaturecheck":      SignatureCheck,
	"utwrite":             UTWrite,
	"utbootconfiguration": UTBootConfiguration,
	"criticalservices":    CriticalServices,
	"inbm":                INBM,
	"oscommit":            OSCommit,
}

// DetailLogEntry represents a single entry in the detail log.
type DetailLogEntry struct {
	UpdateType    UpdateType    `json:"update_type"`
	PackageName   string        `json:"package_name"`
	UpdateTime    time.Time     `json:"update_time"`
	Action        Action        `json:"action"`
	Status        Status        `json:"status"`
	Version       string        `json:"version"`
	FailureReason FailureReason `json:"failure_reason"`
	FailureLog    string        `json:"failure_log"`
}

// DetailLog represents a collection of detail log entries.
type DetailLog struct {
	UpdateLog []DetailLogEntry `json:"update_log"`
}

// UnmarshalJSON implements the json.Unmarshaler interface for UpdateType.
func (ut *UpdateType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	value, ok := stringToUpdateType[str]
	if !ok {
		return fmt.Errorf("unknown UpdateType string: %s", str)
	}
	*ut = value
	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for Action.
func (a *Action) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	value, ok := stringToAction[str]
	if !ok {
		return fmt.Errorf("unknown Action string: %s", str)
	}
	*a = value
	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for Status.
func (s *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	value, ok := stringToStatus[str]
	if !ok {
		return fmt.Errorf("unknown Status string: %s", str)
	}
	*s = value
	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for FailureReason.
func (s *FailureReason) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	value, ok := stringToFailureReason[str]
	if !ok {
		return fmt.Errorf("unknown FailureReason string: %s", str)
	}
	*s = value
	return nil
}
