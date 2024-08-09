// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package unit

import "fmt"

type Prefix interface {
	fmt.Stringer
	Scale() float64
}

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

var MetricPrefixes = []MetricPrefix{MetricPrefixKilo, MetricPrefixMega, MetricPrefixGiga, MetricPrefixTera}

// Scale returns the scale from the base unit or -1 if invalid.
func (m MetricPrefix) Scale() float64 {
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

func (m MetricPrefix) String() string {
	return string(m)
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

var BinaryPrefixes = []BinaryPrefix{BinaryPrefixKibi, BinaryPrefixMebi, BinaryPrefixGibi, BinaryPrefixTebi}

// Scale returns the scale from the base unit or -1 if invalid.
func (b BinaryPrefix) Scale() float64 {
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

func (b BinaryPrefix) String() string {
	return string(b)
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
	scale := binaryPrefix.Scale() / metricPrefix.Scale()
	return metricPrefix, scale, nil
}
