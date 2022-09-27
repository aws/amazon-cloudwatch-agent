// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// OTEL supports: https://unitsofmeasure.org/ucum
// CloudWatch supports:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
var unitMap = map[string]types.StandardUnit{
	"":   types.StandardUnitNone,
	"1":  types.StandardUnitNone,
	"s":  types.StandardUnitSeconds,
	"us": types.StandardUnitMicroseconds,
	"ms": types.StandardUnitMilliseconds,
	// days, hours, minutes, nanoseconds will require a value conversion.
	"B":   types.StandardUnitBytes,
	"By":  types.StandardUnitBytes,
	"KB":  types.StandardUnitKilobytes,
	"KBy": types.StandardUnitKilobytes,
	"MB":  types.StandardUnitMegabytes,
	"MBy": types.StandardUnitMegabytes,
	"GB":  types.StandardUnitGigabytes,
	"GBy": types.StandardUnitGigabytes,
	"TB":  types.StandardUnitTerabytes,
	"TBy": types.StandardUnitTerabytes,
	// kibibytes, mebibytes, etc. will require a value conversion.
	"Bi":  types.StandardUnitBits,
	"KBi": types.StandardUnitKilobits,
	"MBi": types.StandardUnitMegabits,
	"TBi": types.StandardUnitTerabits,
	// rates
	"B/s":   types.StandardUnitBytesSecond,
	"By/s":  types.StandardUnitBytesSecond,
	"KB/s":  types.StandardUnitKilobytesSecond,
	"KBy/s": types.StandardUnitKilobytesSecond,
	"MB/s":  types.StandardUnitMegabytesSecond,
	"MBy/s": types.StandardUnitMegabytesSecond,
	"GB/s":  types.StandardUnitGigabytesSecond,
	"GBy/s": types.StandardUnitGigabytesSecond,
	"TB/s":  types.StandardUnitTerabytesSecond,
	"TBy/s": types.StandardUnitTerabytesSecond,

	"Bi/s":  types.StandardUnitBitsSecond,
	"KBi/s": types.StandardUnitKilobitsSecond,
	"MBi/s": types.StandardUnitMegabitsSecond,
	"GBi/s": types.StandardUnitGigabitsSecond,
	"TBi/s": types.StandardUnitTerabitsSecond,
}

// ConvertUnit converts from the OTEL unit names to the corresponding names
// supported by AWS CloudWatch. Some OTEL unit types are unsupported.
// Some could be supported if we converted the metric value as well.
// For example OTEL could have "KiBy" (kibibytes) with a value of 1.
// We would need to report 1024/1000 to AWS with unit of kilobytes.
// Or leave the value as-is and use "kilobytes" to mean 1000 Bytes and 1024.
func ConvertUnit(unit string) string {
	newUnit, ok := unitMap[unit]
	if ok {
		return string(newUnit)
	}
	return unit
}
