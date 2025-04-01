// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package telemetrymgr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/telemetry/v1"
	telemetrymgr "github.com/open-edge-platform/infra-managers/telemetry/internal/telemetrymgrsvc"
	pb "github.com/open-edge-platform/infra-managers/telemetry/pkg/api/telemetrymgr/v1"
)

func TestDeduplicate(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name     string
		input    []telemetrymgr.TelemetryResources
		expected []telemetrymgr.TelemetryResources
	}{
		{
			name: "Test case 1: Deduplicate resources",
			input: []telemetrymgr.TelemetryResources{
				{
					MetricGroup:    "group1",
					MetricType:     "type1",
					MetricKind:     "kind1",
					LogSeverity:    1,
					MetricInterval: 10,
				},
				{
					MetricGroup:    "group1",
					MetricType:     "type1",
					MetricKind:     "kind1",
					LogSeverity:    0,
					MetricInterval: 5,
				},
			},
			expected: []telemetrymgr.TelemetryResources{
				{
					MetricGroup:    "group1",
					MetricType:     "type1",
					MetricKind:     "kind1",
					LogSeverity:    0,
					MetricInterval: 5,
				},
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := telemetrymgr.Deduplicate(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

//nolint:dupl // this function assign kind
func TestAssignTelemetryResourceKind(t *testing.T) {
	testCases := []struct {
		name     string
		input    int32
		expected pb.TelemetryResourceKind
	}{
		{
			name:     "Unspecified resource kind",
			input:    int32(telemetryv1.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_UNSPECIFIED),
			expected: pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_UNSPECIFIED,
		},
		{
			name:     "Metrics resource kind",
			input:    int32(telemetryv1.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_METRICS),
			expected: pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_METRICS,
		},
		{
			name:     "Logs resource kind",
			input:    int32(telemetryv1.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_LOGS),
			expected: pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_LOGS,
		},
		{
			name:     "Invalid resource kind",
			input:    999, // Some invalid value not defined in the enum
			expected: pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_UNSPECIFIED,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := telemetrymgr.AssignTelemetryResourceKind(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

//nolint:dupl // this function assign collector
func TestAssignTelemetryCollectorKind(t *testing.T) {
	testCases := []struct {
		name     string
		input    int32
		expected pb.CollectorKind
	}{
		{
			name:     "Unspecified collector kind",
			input:    int32(telemetryv1.CollectorKind_COLLECTOR_KIND_UNSPECIFIED),
			expected: pb.CollectorKind_COLLECTOR_KIND_UNSPECIFIED,
		},
		{
			name:     "Host collector kind",
			input:    int32(telemetryv1.CollectorKind_COLLECTOR_KIND_HOST),
			expected: pb.CollectorKind_COLLECTOR_KIND_HOST,
		},
		{
			name:     "Cluster collector kind",
			input:    int32(telemetryv1.CollectorKind_COLLECTOR_KIND_CLUSTER),
			expected: pb.CollectorKind_COLLECTOR_KIND_CLUSTER,
		},
		{
			name:     "Invalid collector kind",
			input:    999, // Some invalid value not defined in the enum
			expected: pb.CollectorKind_COLLECTOR_KIND_UNSPECIFIED,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := telemetrymgr.AssignTelemetryCollectorKind(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestAssignTelemetryResourceSeverity(t *testing.T) {
	testCases := []struct {
		name     string
		input    int32
		expected pb.SeverityLevel
	}{
		{
			name:     "Unspecified severity level",
			input:    int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_UNSPECIFIED),
			expected: pb.SeverityLevel_SEVERITY_LEVEL_UNSPECIFIED,
		},
		{
			name:     "Critical severity level",
			input:    int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_CRITICAL),
			expected: pb.SeverityLevel_SEVERITY_LEVEL_CRITICAL,
		},
		{
			name:     "Error severity level",
			input:    int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_ERROR),
			expected: pb.SeverityLevel_SEVERITY_LEVEL_ERROR,
		},
		{
			name:     "Warn severity level",
			input:    int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_WARN),
			expected: pb.SeverityLevel_SEVERITY_LEVEL_WARN,
		},
		{
			name:     "Info severity level",
			input:    int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_INFO),
			expected: pb.SeverityLevel_SEVERITY_LEVEL_INFO,
		},
		{
			name:     "Debug severity level",
			input:    int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_DEBUG),
			expected: pb.SeverityLevel_SEVERITY_LEVEL_DEBUG,
		},
		{
			name:     "Invalid severity level",
			input:    999, // Some invalid value not defined in the enum
			expected: pb.SeverityLevel_SEVERITY_LEVEL_UNSPECIFIED,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := telemetrymgr.AssignTelemetryResourceSeverity(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestByMetricsSwap(t *testing.T) {
	// Create a slice with two elements
	resources := telemetrymgr.ByMetrics{
		telemetrymgr.TelemetryResources{MetricGroup: "group1", MetricType: "type1", MetricKind: "kind1"},
		telemetrymgr.TelemetryResources{MetricGroup: "group2", MetricType: "type2", MetricKind: "kind2"},
	}

	// Swap the elements
	resources.Swap(0, 1)

	// Verify that the elements have been swapped
	assert.Equal(t, "group2", resources[0].MetricGroup, "The first element should now be 'group2'")
	assert.Equal(t, "group1", resources[1].MetricGroup, "The second element should now be 'group1'")
}
