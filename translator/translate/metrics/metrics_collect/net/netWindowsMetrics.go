// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package net

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

const ObjectName = "Network Interface(*)"

var NetWindowsMetrics []interface{}

func init() {
	pc14 := translator.InitWindowsObject(ObjectName, "*", "Bytes Received/sec", "pc14")
	pc15 := translator.InitWindowsObject(ObjectName, "*", "Bytes Sent/sec", "pc15")
	pc16 := translator.InitWindowsObject(ObjectName, "*", "Packets Received Errors", "pc16")
	pc17 := translator.InitWindowsObject(ObjectName, "*", "Packets Received Discarded", "pc17")
	pc18 := translator.InitWindowsObject(ObjectName, "*", "Packets Received Unknown", "pc18")
	NetWindowsMetrics = append(NetWindowsMetrics, pc14)
	NetWindowsMetrics = append(NetWindowsMetrics, pc15)
	NetWindowsMetrics = append(NetWindowsMetrics, pc16)
	NetWindowsMetrics = append(NetWindowsMetrics, pc17)
	NetWindowsMetrics = append(NetWindowsMetrics, pc18)
}
