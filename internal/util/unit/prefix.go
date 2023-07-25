// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package unit

import "fmt"

// MetricPrefix is a base 10 prefix used by the metric system.
type MetricPrefix string

const (
	kilo float64 = 1e3
	mega         = 1e6
	giga         = 1e9
	tera         = 1e12

	MetricPrefixKilo MetricPrefix = "k"
	MetricPrefixMega              = "M"
	MetricPrefixGiga              = "G"
	MetricPrefixTera              = "T"
)

// Value returns the scale from the base unit or -1 if invalid.
func (m MetricPrefix) Value() float64 {
	switch m {
	case MetricPrefixKilo:
		return kilo
	case MetricPrefixMega:
		return mega
	case MetricPrefixGiga:
		return giga
	case MetricPrefixTera:
		return tera
	}
	return -1
}

// BinaryPrefix is a base 2 prefix for data storage.
type BinaryPrefix string

const (
	_            = iota
	kibi float64 = 1 << (10 * iota)
	mebi
	gibi
	tebi

	BinaryPrefixKibi BinaryPrefix = "Ki"
	BinaryPrefixMebi              = "Mi"
	BinaryPrefixGibi              = "Gi"
	BinaryPrefixTebi              = "Ti"
)

// Value returns the scale from the base unit or -1 if invalid.
func (b BinaryPrefix) Value() float64 {
	switch b {
	case BinaryPrefixKibi:
		return kibi
	case BinaryPrefixMebi:
		return mebi
	case BinaryPrefixGibi:
		return gibi
	case BinaryPrefixTebi:
		return tebi
	}
	return -1
}

var binaryToMetricMapping = map[BinaryPrefix]MetricPrefix{
	BinaryPrefixKibi: MetricPrefixKilo,
	BinaryPrefixMebi: MetricPrefixMega,
	BinaryPrefixGibi: MetricPrefixGiga,
	BinaryPrefixTebi: MetricPrefixTera,
}

// ConvertToMetric returns the mapped metric prefix and a scale factor for adjusting values.
func ConvertToMetric(binaryPrefix BinaryPrefix) (MetricPrefix, float64, error) {
	metricPrefix, ok := binaryToMetricMapping[binaryPrefix]
	if !ok {
		return "", -1, fmt.Errorf("no valid conversion for %v", binaryPrefix)
	}
	scale := binaryPrefix.Value() / metricPrefix.Value()
	return metricPrefix, scale, nil
}
