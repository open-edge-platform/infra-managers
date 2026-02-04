// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package attestmgr_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	statusv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/status/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	attestmgr_sb "github.com/open-edge-platform/infra-managers/attestationstatus/pkg/api/attestmgr/v1"
)

const (
	dummyGUID = "99FE0C5F-8F58-4082-9F66-16C181E2E03B"
	tenant1   = "11111111-1111-1111-1111-111111111111"
)

func TestAttestationStatusMgrClient_UpdateInstanceAttestationStatusByHostGuid(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	// fail with no JWT (comes from context)
	badJWT := &attestmgr_sb.UpdateInstanceAttestationStatusByHostGuidRequest{
		HostGuid: dummyGUID,
	}

	respGet, err := AttestMgrTestClient.UpdateInstanceAttestationStatusByHostGuid(t.Context(), badJWT)
	require.Error(t, err)
	require.Nil(t, respGet)

	// fail with bad GUID
	badGUID := &attestmgr_sb.UpdateInstanceAttestationStatusByHostGuidRequest{
		HostGuid: "invalid",
	}

	respGet, err = AttestMgrTestClient.UpdateInstanceAttestationStatusByHostGuid(ctx, badGUID)
	require.Error(t, err)
	require.Nil(t, respGet)

	// fail by trying to set UNSPECIFIED as the status
	badStatus := &attestmgr_sb.UpdateInstanceAttestationStatusByHostGuidRequest{
		HostGuid:          dummyGUID,
		AttestationStatus: attestmgr_sb.AttestationStatus_ATTESTATION_STATUS_UNSPECIFIED,
	}

	respGet, err = AttestMgrTestClient.UpdateInstanceAttestationStatusByHostGuid(ctx, badStatus)
	require.Error(t, err)
	require.Nil(t, respGet)

	// create test host in Inventory
	hostInv1 := dao.CreateHost(t, tenant1)

	// fail by not having an Instance associated with Host
	noInstance := &attestmgr_sb.UpdateInstanceAttestationStatusByHostGuidRequest{
		HostGuid:          hostInv1.GetUuid(),
		AttestationStatus: attestmgr_sb.AttestationStatus_ATTESTATION_STATUS_VERIFIED,
	}

	respGet, err = AttestMgrTestClient.UpdateInstanceAttestationStatusByHostGuid(ctx, noInstance)
	require.Error(t, err)
	require.Nil(t, respGet)

	// create os and instance in Inventory
	osInv := dao.CreateOs(t, tenant1)
	instInv1 := dao.CreateInstance(t, tenant1, hostInv1, osInv)

	// test an Attestation Verified request
	reqGood := &attestmgr_sb.UpdateInstanceAttestationStatusByHostGuidRequest{
		HostGuid:          hostInv1.GetUuid(),
		AttestationStatus: attestmgr_sb.AttestationStatus_ATTESTATION_STATUS_VERIFIED,
	}

	respGet, err = AttestMgrTestClient.UpdateInstanceAttestationStatusByHostGuid(ctx, reqGood)
	require.NoError(t, err)
	require.NotNil(t, respGet)

	// Validate that SB Verified status translates to Inventory status Idle
	inst1 := GetInstanceByResourceID(t, instInv1.GetResourceId())
	assert.Equal(t, inst1.TrustedAttestationStatusIndicator, statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	assert.Equal(t, inst1.TrustedAttestationStatus, "Verified")
	// Unix timestamps are always positive, so conversion from int64 to uint64 is safe
	now := time.Now().Unix()
	assert.GreaterOrEqual(t, uint64(now), inst1.TrustedAttestationStatusTimestamp)

	// create another host and instance in Inventory
	hostInv2 := dao.CreateHost(t, tenant1)
	instInv2 := dao.CreateInstance(t, tenant1, hostInv2, osInv)

	// test a Attestation Failed request
	reqAttFail := &attestmgr_sb.UpdateInstanceAttestationStatusByHostGuidRequest{
		HostGuid:                hostInv2.GetUuid(),
		AttestationStatus:       attestmgr_sb.AttestationStatus_ATTESTATION_STATUS_FAILED,
		AttestationStatusDetail: "Failed to Attest",
	}

	respGet, err = AttestMgrTestClient.UpdateInstanceAttestationStatusByHostGuid(ctx, reqAttFail)
	require.NoError(t, err)
	require.NotNil(t, respGet)

	// Validate that SB Failed status translates to Inventory status Error, and detail passed
	inst2 := GetInstanceByResourceID(t, instInv2.GetResourceId())
	assert.Equal(t, inst2.TrustedAttestationStatusIndicator, statusv1.StatusIndication_STATUS_INDICATION_ERROR)
	assert.Equal(t, inst2.TrustedAttestationStatus, reqAttFail.AttestationStatusDetail)
}
