/*
* SPDX-FileCopyrightText: (C) 2025 Intel Corporation
* SPDX-License-Identifier: Apache-2.0
 */

// Package telemetrymgr provides telemetry manager service implementation.
package telemetrymgr

import (
	"sort"

	telemetryv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/telemetry/v1"
	pb "github.com/open-edge-platform/infra-managers/telemetry/pkg/api/telemetrymgr/v1"
)

// TelemetryResources represents telemetry resource configuration for agent mapping.
type TelemetryResources struct {
	MetricGroup    string
	MetricType     string
	MetricKind     string
	LogSeverity    uint32
	MetricInterval uint32
}

// ByMetrics is a custom type to implement sorting by MetricGroups, MetricType, and MetricKind.
type ByMetrics []TelemetryResources

func (a ByMetrics) Len() int      { return len(a) }
func (a ByMetrics) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByMetrics) Less(i, j int) bool {
	if a[i].MetricGroup != a[j].MetricGroup {
		return a[i].MetricGroup < a[j].MetricGroup
	}
	if a[i].MetricType != a[j].MetricType {
		return a[i].MetricType < a[j].MetricType
	}
	return a[i].MetricKind < a[j].MetricKind
}

func getLastIndex(resources []TelemetryResources, index int) int {
	lastIndex := index
	for j := index + 1; j < len(resources); j++ {
		if resources[j].MetricGroup != resources[index].MetricGroup ||
			resources[j].MetricType != resources[index].MetricType ||
			resources[j].MetricKind != resources[index].MetricKind {
			break
		}
		lastIndex = j
	}
	return lastIndex
}

// Deduplicate removes duplicate telemetry resources and combines them based on priority.
func Deduplicate(resources []TelemetryResources) []TelemetryResources {
	sort.Sort(ByMetrics(resources))

	var deduplicated []TelemetryResources
	for i := 0; i < len(resources); i++ {
		// Find the last occurrence of each combination of metric_groups, metric_type, metric_Kind
		lastIndex := getLastIndex(resources, i)

		if resources[i].MetricType == telemetryv1.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_LOGS.String() {
			// Keep the one with the highest logSeverity
			maxLevel := resources[i].LogSeverity
			for j := i + 1; j <= lastIndex; j++ {
				if resources[j].LogSeverity > maxLevel {
					maxLevel = resources[j].LogSeverity
				}
			}

			// Add the deduplicated entry
			deduplicated = append(deduplicated, TelemetryResources{
				MetricGroup: resources[i].MetricGroup,
				MetricType:  resources[i].MetricType,
				MetricKind:  resources[i].MetricKind,
				LogSeverity: maxLevel,
			})
		} else {
			// Keep the one with the lowest metric_interval
			minInterval := resources[i].MetricInterval
			for j := i + 1; j <= lastIndex; j++ {
				if resources[j].MetricInterval < minInterval {
					minInterval = resources[j].MetricInterval
				}
			}

			// Add the deduplicated entry
			deduplicated = append(deduplicated, TelemetryResources{
				MetricGroup:    resources[i].MetricGroup,
				MetricType:     resources[i].MetricType,
				MetricKind:     resources[i].MetricKind,
				MetricInterval: minInterval,
			})
		}

		// Move to the next set of different metric_groups, metric_type, metric_Kind
		i = lastIndex
	}

	return deduplicated
}

// AssignTelemetryResourceKind assigns the telemetry resource kind.
func AssignTelemetryResourceKind(number int32) pb.TelemetryResourceKind {
	switch number {
	case int32(telemetryv1.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_UNSPECIFIED):
		return pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_UNSPECIFIED
	case int32(telemetryv1.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_METRICS):
		return pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_METRICS
	case int32(telemetryv1.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_LOGS):
		return pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_LOGS
	default:
		return pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_UNSPECIFIED
	}
}

// AssignTelemetryCollectorKind assigns the telemetry collector kind.
func AssignTelemetryCollectorKind(number int32) pb.CollectorKind {
	switch number {
	case int32(telemetryv1.CollectorKind_COLLECTOR_KIND_UNSPECIFIED):
		return pb.CollectorKind_COLLECTOR_KIND_UNSPECIFIED
	case int32(telemetryv1.CollectorKind_COLLECTOR_KIND_HOST):
		return pb.CollectorKind_COLLECTOR_KIND_HOST
	case int32(telemetryv1.CollectorKind_COLLECTOR_KIND_CLUSTER):
		return pb.CollectorKind_COLLECTOR_KIND_CLUSTER
	default:
		return pb.CollectorKind_COLLECTOR_KIND_UNSPECIFIED
	}
}

// AssignTelemetryResourceSeverity assigns the telemetry resource severity level.
func AssignTelemetryResourceSeverity(number int32) pb.SeverityLevel {
	switch number {
	case int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_UNSPECIFIED):
		return pb.SeverityLevel_SEVERITY_LEVEL_UNSPECIFIED
	case int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_CRITICAL):
		return pb.SeverityLevel_SEVERITY_LEVEL_CRITICAL
	case int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_ERROR):
		return pb.SeverityLevel_SEVERITY_LEVEL_ERROR
	case int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_WARN):
		return pb.SeverityLevel_SEVERITY_LEVEL_WARN
	case int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_INFO):
		return pb.SeverityLevel_SEVERITY_LEVEL_INFO
	case int32(telemetryv1.SeverityLevel_SEVERITY_LEVEL_DEBUG):
		return pb.SeverityLevel_SEVERITY_LEVEL_DEBUG
	default:
		return pb.SeverityLevel_SEVERITY_LEVEL_UNSPECIFIED
	}
}
