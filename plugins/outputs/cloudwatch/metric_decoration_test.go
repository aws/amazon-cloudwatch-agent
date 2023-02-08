// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMetricDecorations(t *testing.T) {
	expected := make([]MetricDecorationConfig, 0)

	mdc := MetricDecorationConfig{
		Category: "cpu",
		Metric:   "cpu",
		Rename:   "CPU",
		Unit:     "Percent",
	}
	expected = append(expected, mdc)

	mdc = MetricDecorationConfig{
		Category: "mem",
		Metric:   "mem",
		Unit:     "Megabytes",
	}
	expected = append(expected, mdc)

	mdc = MetricDecorationConfig{
		Category: "disk",
		Metric:   "disk",
		Rename:   "DISK",
	}
	expected = append(expected, mdc)

	m, err := NewMetricDecorations(expected)
	assert.True(t, err == nil)

	assert.Equal(t, "CPU", m.getRename("cpu", "cpu"))
	assert.Equal(t, "Percent", m.getUnit("cpu", "cpu"))
	assert.Equal(t, "Megabytes", m.getUnit("mem", "mem"))
	assert.Equal(t, "DISK", m.getRename("disk", "disk"))
}

func TestNewMetricDecorationsAbnormal(t *testing.T) {
	expected := make([]MetricDecorationConfig, 0)

	mdc := MetricDecorationConfig{
		Category: "cpu",
		Metric:   "cpu",
		Rename:   "CPU",
		Unit:     "InvalidUnit",
	}
	expected = append(expected, mdc)

	_, err := NewMetricDecorations(expected)
	assert.True(t, err != nil)

	_, err = NewMetricDecorations(nil)
	assert.True(t, err == nil)
}

func TestNewMetricDecorationsSpecialCharacter(t *testing.T) {
	expected := make([]MetricDecorationConfig, 0)

	mdc := MetricDecorationConfig{
		Category: "/cpu",
		Metric:   "% cpu",
		Rename:   "\\CPU",
	}
	expected = append(expected, mdc)

	m, err := NewMetricDecorations(expected)
	assert.True(t, err == nil)
	assert.Equal(t, "\\CPU", m.getRename("/cpu", "% cpu"))
}
