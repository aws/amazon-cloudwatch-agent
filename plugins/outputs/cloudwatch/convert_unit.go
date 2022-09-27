// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

// CloudWatch supports:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
var units = map[string]string{
	"":   "None",
	"1":  "None",
	"s":  "Seconds",
	"us": "Microseconds",
	"ms": "Milliseconds",
	// days, hours, minutes, nanoseconds will require a value conversion.
	"B":   "Bytes",
	"By":  "Bytes",
	"KB":  "Kilobytes",
	"KBy": "Kilobytes",
	"MB":  "Megabytes",
	"MBy": "Megabytes",
	"GB":  "Gigabytes",
	"GBy": "Gigabytes",
	"TB":  "Terabytes",
	"TBy": "Terabytes",
	// kibibytes, mebibytes, etc. will require a value conversion.
	"Bi":  "Bits",
	"KBi": "Kilobits",
	"MBi": "Megabits",
	"TBi": "Terabits",
	// rates
	"B/s":   "Bytes/Second",
	"By/s":  "Bytes/Second",
	"KB/s":  "Kilobytes/Second",
	"KBy/s": "Kilobytes/Second",
	"MB/s":  "Megabytes/Second",
	"MBy/s": "Megabytes/Second",
	"GB/s":  "Gigabytes/Second",
	"GBy/s": "Gigabytes/Second",
	"TB/s":  "Terabytes/Second",
	"TBy/s": "Terabytes/Second",

	"Bi/s":  "Bits/Second",
	"KBi/s": "Kilobits/Second",
	"MBi/s": "Megabits/Second",
	"GBi/s": "Gigabits/Second",
	"TBi/s": "Terabits/Second",
}

// ConvertUnit converts from the OTEL unit names to the corresponding names
// supported by AWS CloudWatch. Some OTEL unit types are unsupported.
// Some could be supported if we converted the metric value as well.
// For example OTEL could have "KiBy" (kibibytes) with a value of 1.
// We would need to report 1024/1000 to AWS with unit of kilobytes.
// Or leave the value as-is and use "kilobytes" to mean 1000 Bytes and 1024.
func ConvertUnit(unit string) string {
	u, ok := units[unit]
	if ok {
		return u
	}
	return unit
}
