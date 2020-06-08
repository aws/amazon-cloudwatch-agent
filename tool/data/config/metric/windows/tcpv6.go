package windows

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type TCPv6 struct {
	ConnectionsEstablished bool `Connections Established`
}

func (config *TCPv6) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}
	measurement := []string{}
	if config.ConnectionsEstablished {
		measurement = append(measurement, "Connections Established")
	}
	resultMap[util.MapKeyMeasurement] = measurement
	return "TCPv6", resultMap
}

func (config *TCPv6) Enable() {
	config.ConnectionsEstablished = true
}
