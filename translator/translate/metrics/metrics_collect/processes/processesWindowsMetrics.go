// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package processes

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

const ObjectName = "Process(*)"

var ProcessesWindowsMetrics []interface{}

func init() {
	pc21 := translator.InitWindowsObject(ObjectName, "*", "% Processor Time", "pc21")
	ProcessesWindowsMetrics = append(ProcessesWindowsMetrics, pc21)
}
