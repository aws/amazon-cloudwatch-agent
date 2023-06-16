// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cpu

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

const ObjectName = "Processor(*)"

var CpuWindowsMetrics []interface{}

func init() {
	pc1 := translator.InitWindowsObject(ObjectName, "*", "% Processor Time", "pc1")
	pc2 := translator.InitWindowsObject(ObjectName, "*", "% User Time", "pc2")
	pc3 := translator.InitWindowsObject(ObjectName, "*", "% Privileged Time", "pc3")
	pc4 := translator.InitWindowsObject(ObjectName, "*", "Interrupts/sec", "pc4")
	pc5 := translator.InitWindowsObject(ObjectName, "*", "% Idle Time", "pc5")
	pc6 := translator.InitWindowsObject(ObjectName, "*", "% Interrupt Time", "pc6")
	CpuWindowsMetrics = append(CpuWindowsMetrics, pc1)
	CpuWindowsMetrics = append(CpuWindowsMetrics, pc2)
	CpuWindowsMetrics = append(CpuWindowsMetrics, pc3)
	CpuWindowsMetrics = append(CpuWindowsMetrics, pc4)
	CpuWindowsMetrics = append(CpuWindowsMetrics, pc5)
	CpuWindowsMetrics = append(CpuWindowsMetrics, pc6)
}
