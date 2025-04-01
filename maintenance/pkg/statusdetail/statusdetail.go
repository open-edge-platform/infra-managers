// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package statusdetail

import (
	"encoding/json"
	"fmt"
	"time"
)

type (
	Status        int
	Action        int
	UpdateType    int
	FailureReason int
)

const (
	Installed Status = iota
	Failed
	Rolledback
)

const (
	Upgrade Action = iota
	Install
)

const (
	OS UpdateType = iota
	Application
	Firmware
	Container
	Configuration
)

const (
	Unspecified FailureReason = iota
	NoFailure
	Download
	Installation
	Bootloader
	InsufficientStorage
	RSAuthentication
	SignatureCheck
	UTWrite
	UTBootConfiguration
	CriticalServices
	INBM
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

type DetailLog struct {
	UpdateLog []DetailLogEntry `json:"update_log"`
}

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
