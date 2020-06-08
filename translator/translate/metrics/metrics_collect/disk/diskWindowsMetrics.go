package disk

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const ObjectName = "PhysicalDisk(*)"

var DiskWindowsMetrics []interface{}

func init() {
	pc11 := translator.InitWindowsObject(ObjectName, "*", "Disk Reads/sec", "pc11")
	pc12 := translator.InitWindowsObject(ObjectName, "*", "Disk Writes/sec", "pc12")
	pc13 := translator.InitWindowsObject(ObjectName, "*", "% Idle Time", "pc13")
	DiskWindowsMetrics = append(DiskWindowsMetrics, pc11)
	DiskWindowsMetrics = append(DiskWindowsMetrics, pc12)
	DiskWindowsMetrics = append(DiskWindowsMetrics, pc13)
}
