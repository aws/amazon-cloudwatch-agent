// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mem

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const ObjectName = "Memory"

var MemWindowsMetrics []interface{}

func init() {
	pc7 := translator.InitWindowsObject(ObjectName, "*", "Page Faults/sec", "pc7")
	pc8 := translator.InitWindowsObject(ObjectName, "*", "Pages/sec", "pc8")
	pc9 := translator.InitWindowsObject(ObjectName, "*", "Available MBytes", "pc9")
	pc10 := translator.InitWindowsObject(ObjectName, "*", "Page Writes/sec", "pc10")
	MemWindowsMetrics = append(MemWindowsMetrics, pc7)
	MemWindowsMetrics = append(MemWindowsMetrics, pc8)
	MemWindowsMetrics = append(MemWindowsMetrics, pc9)
	MemWindowsMetrics = append(MemWindowsMetrics, pc10)
}
