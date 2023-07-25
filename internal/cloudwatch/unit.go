// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"fmt"
	"time"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
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
	"B":  types.StandardUnitBytes,
	"By": types.StandardUnitBytes,
	"Bi": types.StandardUnitBits,
	// rates
	"B/s":  types.StandardUnitBytesSecond,
	"By/s": types.StandardUnitBytesSecond,
	"Bi/s": types.StandardUnitBitsSecond,
}

var uniqueConversions = map[string]struct {
	unit  types.StandardUnit
	scale float64
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
	if IsStandardUnit(unit) {
		return unit, 1, nil
	}
	if baseUnit, ok := baseUnits[unit]; ok {
		return string(baseUnit), 1, nil
	}
	if conversion, ok := uniqueConversions[unit]; ok {
		return string(conversion.unit), conversion.scale, nil
	}
	prefix, base := splitUnit(unit)
	if baseUnit, ok := baseUnits[base]; ok {
		return scaleBaseUnit(prefix, baseUnit)
	}
	return string(types.StandardUnitNone), 1, fmt.Errorf("non-convertible unit: %q", unit)
}

// splitUnit splits a unit and its prefix based on the second capital letter found.
// e.g. MiBy will split into prefix "Mi" and base "By".
func splitUnit(unit string) (string, string) {
	var index int
	if len(unit) > 1 {
		for i, r := range unit[1:] {
			if unicode.IsUpper(r) {
				index = i + 1
				break
			}
		}
	}
	return unit[:index], unit[index:]
}

// scaleBaseUnit takes a prefix and the CloudWatch base unit and finds the scaled CloudWatch unit and
// the scale factor if value adjustments are necessary.
func scaleBaseUnit(prefix string, baseUnit types.StandardUnit) (string, float64, error) {
	scaledUnits, ok := scaledBaseUnits[baseUnit]
	if !ok {
		return string(types.StandardUnitNone), 1, fmt.Errorf("non-scalable unit: %v", baseUnit)
	}
	scale := float64(1)
	metricPrefix := unit.MetricPrefix(prefix)
	if metricPrefix.Value() == -1 {
		var err error
		metricPrefix, scale, err = unit.ConvertToMetric(unit.BinaryPrefix(prefix))
		if err != nil {
			return string(types.StandardUnitNone), 1, fmt.Errorf("unsupported prefix: %v", prefix)
		}
	}
	if scaledUnit, ok := scaledUnits[metricPrefix]; ok {
		return string(scaledUnit), scale, nil
	}
	return string(types.StandardUnitNone), 1, fmt.Errorf("unsupported prefix %v for %v", prefix, baseUnit)
}

var standardUnits = collections.NewSet[string]()

// IsStandardUnit determines if the unit is acceptable by CloudWatch.
func IsStandardUnit(unit string) bool {
	if unit == "" {
		return false
	}
	_, ok := standardUnits[unit]
	return ok
}

func init() {
	for _, standardUnit := range types.StandardUnitNone.Values() {
		standardUnits.Add(string(standardUnit))
	}
}
