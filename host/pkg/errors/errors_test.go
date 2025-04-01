// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package errors_test

import (
	"testing"

	"google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"

	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-managers/host/pkg/errors"
)

func TestWrap(t *testing.T) {
	testCases := map[string]struct {
		inError error
		outCode codes.Code
		outDesc string
	}{
		"NilError": {
			inError: nil,
			outCode: 0,
			outDesc: "",
		},
		"HandledByInvWrap": {
			inError: inv_errors.Errorfc(codes.Unavailable, "I am not available"),
			outCode: codes.Unavailable,
			outDesc: "I am not available",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			err := errors.Wrap(testCase.inError)

			if testCase.inError == nil && err == nil {
				return
			}

			st := grpc_status.Convert(err)
			// Code validation
			if st.Code() != testCase.outCode {
				t.Errorf("Want Code %s - Got Code %s", testCase.outCode, st.Code())
				return
			}
			// Description validation
			if st.Message() != testCase.outDesc {
				t.Errorf("Want Desc %s - Got Desc %s", testCase.outDesc, st.Message())
				return
			}
		})
	}
}
