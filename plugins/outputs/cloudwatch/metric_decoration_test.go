// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/stretchr/testify/assert"
)

func TestNewMetricDecorations(t *testing.T) {
	metricDecoration := []MetricDecorationConfig{
		{
			Category: "cpu",
			Metric:   "cpu",
			Rename:   "CPU",
			Unit:     "Percent",
		},
		{
			Category: "mem",
			Metric:   "mem",
			Unit:     "Megabytes",
		},
		{
			Category: "disk",
			Metric:   "disk",
			Rename:   "DISK",
		},
	}

	m, err := NewMetricDecorations(metricDecoration)
	assert.NoError(t, err)

	assert.Equal(t, "CPU", m.getRename("cpu", "cpu"))
	assert.Equal(t, "Percent", m.getUnit("cpu", "cpu"))
	assert.Equal(t, "Megabytes", m.getUnit("mem", "mem"))
	assert.Equal(t, "DISK", m.getRename("disk", "disk"))
}

func TestNewMetricDecorationsAbnormal(t *testing.T) {
	metricDecoration := []MetricDecorationConfig{
		{
			Category: "cpu",
			Metric:   "cpu",
			Rename:   "CPU",
			Unit:     "InvalidUnit",
		},
	}

	_, err := NewMetricDecorations(metricDecoration)
	assert.True(t, err != nil)

	_, err = NewMetricDecorations(nil)
	assert.True(t, err == nil)
}

func TestNewMetricDecorationsSpecialCharacter(t *testing.T) {
	metricDecoration := []MetricDecorationConfig{
		{Category: "/cpu",
			Metric: "% cpu",
			Rename: "\\CPU"},
	}

	m, err := NewMetricDecorations(metricDecoration)
	assert.NoError(t, err)
	assert.Equal(t, "\\CPU", m.getRename("/cpu", "% cpu"))
}

func TestOverrideDefaultUnit(t *testing.T) {
	m, err := NewMetricDecorations(nil)

	assert.NoError(t, err)
	assert.Equal(t, "Percent", m.getUnit("cpu", "usage_idle"))

	expectedMetricDecoration := []MetricDecorationConfig{
		{
			Category: "cpu",
			Metric:   "usage_idle",
			Unit:     "Bytes",
		},
		{
			Category: "Network Interface",
			Metric:   "Packets Sent/sec",
			Unit:     "Bytes",
		},
	}

	m, err = NewMetricDecorations(expectedMetricDecoration)
	assert.NoError(t, err)
	assert.Equal(t, "Bytes", m.getUnit("cpu", "usage_idle"))
}

func TestDefaultUnit(t *testing.T) {

	testCases := []struct {
		category            string
		actualMetrics       []string
		expectedMetricsUnit []string
	}{
		{
			category:            "procstat",
			actualMetrics:       []string{"read_bytes", "rlimit_file_locks_soft", "rlimit_memory_rss_soft", "involuntary_context_switches"},
			expectedMetricsUnit: []string{cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitCount},
		},
		{
			category:            "cpu",
			actualMetrics:       []string{"usage_active", "usage_iowait", "usage_user"},
			expectedMetricsUnit: []string{cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent},
		},
		{
			category:            "disk",
			actualMetrics:       []string{"free", "inodes_free", "used_percent"},
			expectedMetricsUnit: []string{cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitPercent},
		},
		{
			category:            "diskio",
			actualMetrics:       []string{"iops_in_progress", "reads", "read_bytes", "write_time"},
			expectedMetricsUnit: []string{cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitMilliseconds},
		},
		{
			category:            "netstat",
			actualMetrics:       []string{"tcp_established", "tcp_last_ack", "udp_socket"},
			expectedMetricsUnit: []string{cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount},
		},
		{
			category:            "processes",
			actualMetrics:       []string{"blocked", "wait", "dead"},
			expectedMetricsUnit: []string{cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount},
		},
		{
			category:            "mem",
			actualMetrics:       []string{"used", "inactive", "used_percent"},
			expectedMetricsUnit: []string{cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitPercent},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Category %s default unit", tc.category), func(_ *testing.T) {
			m, err := NewMetricDecorations(nil)
			assert.NoError(t, err)

			assert.Equal(t, len(tc.actualMetrics), len(tc.expectedMetricsUnit))

			for metricIndex := 0; metricIndex < len(tc.actualMetrics); metricIndex++ {
				actualMetric := tc.actualMetrics[metricIndex]
				expectMetricUnit := tc.expectedMetricsUnit[metricIndex]
				assert.Equal(t, expectMetricUnit, m.getUnit(tc.category, actualMetric))

			}
		})
	}

}

func TestMetricDefaultUnitLength(t *testing.T) {
	for category := range metricDefaultUnit {
		assert.Equal(t, len(metricDefaultUnit[category].supportedMetrics), len(metricDefaultUnit[category].defaultMetricsUnit))
	}
}
