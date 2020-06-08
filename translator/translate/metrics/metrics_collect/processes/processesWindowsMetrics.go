package processes

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const ObjectName = "Process(*)"

var ProcessesWindowsMetrics []interface{}

func init() {
	pc21 := translator.InitWindowsObject(ObjectName, "*", "% Processor Time", "pc21")
	ProcessesWindowsMetrics = append(ProcessesWindowsMetrics, pc21)
}
