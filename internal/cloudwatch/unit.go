// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/unit"
)

// OTEL supports: https://unitsofmeasure.org/ucum
// CloudWatch supports:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
var baseUnits = map[string]types.StandardUnit{
	"":  types.StandardUnitNone,
	"1": types.StandardUnitNone,
	"%": types.StandardUnitPercent,
	// time
	"s":  types.StandardUnitSeconds,
	"us": types.StandardUnitMicroseconds,
	"ms": types.StandardUnitMilliseconds,
	// bytes
	"b":  types.StandardUnitBytes,
	"by": types.StandardUnitBytes,
	"bi": types.StandardUnitBits,
	// rates
	"b/s":  types.StandardUnitBytesSecond,
	"by/s": types.StandardUnitBytesSecond,
	"bi/s": types.StandardUnitBitsSecond,
}

var uniqueConversions = map[string]struct {
	standardUnit types.StandardUnit
	scale        float64
}{
	// time
	"ns":  {types.StandardUnitMicroseconds, 1 / float64(time.Microsecond.Nanoseconds())},
	"min": {types.StandardUnitSeconds, time.Minute.Seconds()},
	"h":   {types.StandardUnitSeconds, time.Hour.Seconds()},
	"d":   {types.StandardUnitSeconds, 24 * time.Hour.Seconds()},
}

var scaledBaseUnits = map[types.StandardUnit]map[unit.MetricPrefix]types.StandardUnit{
	types.StandardUnitBits: {
		unit.MetricPrefixKilo: types.StandardUnitKilobits,
		unit.MetricPrefixMega: types.StandardUnitMegabits,
		unit.MetricPrefixGiga: types.StandardUnitGigabits,
		unit.MetricPrefixTera: types.StandardUnitTerabits,
	},
	types.StandardUnitBytes: {
		unit.MetricPrefixKilo: types.StandardUnitKilobytes,
		unit.MetricPrefixMega: types.StandardUnitMegabytes,
		unit.MetricPrefixGiga: types.StandardUnitGigabytes,
		unit.MetricPrefixTera: types.StandardUnitTerabytes,
	},
	types.StandardUnitBitsSecond: {
		unit.MetricPrefixKilo: types.StandardUnitKilobitsSecond,
		unit.MetricPrefixMega: types.StandardUnitMegabitsSecond,
		unit.MetricPrefixGiga: types.StandardUnitGigabitsSecond,
		unit.MetricPrefixTera: types.StandardUnitTerabitsSecond,
	},
	types.StandardUnitBytesSecond: {
		unit.MetricPrefixKilo: types.StandardUnitKilobytesSecond,
		unit.MetricPrefixMega: types.StandardUnitMegabytesSecond,
		unit.MetricPrefixGiga: types.StandardUnitGigabytesSecond,
		unit.MetricPrefixTera: types.StandardUnitTerabytesSecond,
	},
}

// ToStandardUnit converts from the OTEL unit names to the corresponding names
// supported by AWS CloudWatch. Some OTEL unit types are unsupported.
func ToStandardUnit(unit string) (string, float64, error) {
	standardUnit, scale, err := toStandardUnit(unit)
	return string(standardUnit), scale, err
}

func toStandardUnit(unit string) (types.StandardUnit, float64, error) {
	u := strings.ToLower(unit)
	if standardUnit, ok := standardUnits[u]; ok {
		return standardUnit, 1, nil
	}
	if standardUnit, ok := baseUnits[u]; ok {
		return standardUnit, 1, nil
	}
	if conversion, ok := uniqueConversions[u]; ok {
		return conversion.standardUnit, conversion.scale, nil
	}
	prefix, baseUnit := splitUnit(u)
	if standardUnit, ok := baseUnits[baseUnit]; ok && prefix != nil {
		return scaleBaseUnit(prefix, standardUnit)
	}
	return types.StandardUnitNone, 1, fmt.Errorf("non-convertible unit: %q", unit)
}

// splitUnit splits a unit and its prefix based on available prefixes.
// e.g. MiBy will split into prefix "Mi" and base "By".
func splitUnit(unit string) (unit.Prefix, string) {
	for _, prefix := range supportedPrefixes {
		p := strings.ToLower(prefix.String())
		baseUnit, ok := strings.CutPrefix(unit, p)
		if ok {
			return prefix, baseUnit
		}
	}
	return nil, unit
}

// scaleBaseUnit takes a prefix and the CloudWatch standard unit and finds the scaled CloudWatch unit and
// the scale factor if value adjustments are necessary.
func scaleBaseUnit(prefix unit.Prefix, standardUnit types.StandardUnit) (types.StandardUnit, float64, error) {
	scaledUnits, ok := scaledBaseUnits[standardUnit]
	if !ok {
		return types.StandardUnitNone, 1, fmt.Errorf("non-scalable unit: %v", standardUnit)
	}
	var metricPrefix unit.MetricPrefix
	scale := float64(1)
	switch p := prefix.(type) {
	case unit.MetricPrefix:
		metricPrefix = p
	case unit.BinaryPrefix:
		var err error
		metricPrefix, scale, err = unit.ConvertToMetric(p)
		if err != nil {
			return types.StandardUnitNone, 1, err
		}
	default:
		return types.StandardUnitNone, 1, fmt.Errorf("unsupported prefix: %v", prefix)
	}
	if scaledUnit, ok := scaledUnits[metricPrefix]; ok {
		return scaledUnit, scale, nil
	}
	return types.StandardUnitNone, 1, fmt.Errorf("unsupported prefix %v for %v", prefix, standardUnit)
}

var (
	standardUnits     = make(map[string]types.StandardUnit)
	supportedPrefixes []unit.Prefix
)

func init() {
	for _, standardUnit := range types.StandardUnitNone.Values() {
		standardUnits[strings.ToLower(string(standardUnit))] = standardUnit
	}
	for _, binaryPrefix := range unit.BinaryPrefixes {
		supportedPrefixes = append(supportedPrefixes, binaryPrefix)
	}
	for _, metricPrefix := range unit.MetricPrefixes {
		supportedPrefixes = append(supportedPrefixes, metricPrefix)
	}
}
