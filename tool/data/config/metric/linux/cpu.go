package linux

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type CPU struct {
	PerCore  bool
	TotalCPU bool

	UsageIdle   bool `cpu_usage_idle`
	UsageIOWait bool `cpu_usage_iowait`
	UsageSteal  bool `cpu_usage_steal`
	UsageGuest  bool `cpu_usage_guest`
	UsageUser   bool `cpu_usage_user`
	UsageSystem bool `cpu_usage_system`
}

func (config *CPU) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	config.PerCore = ctx.WantPerInstanceMetrics

	if config.PerCore {
		resultMap[util.MapKeyInstances] = []string{"*"}
	}
	resultMap["totalcpu"] = config.TotalCPU

	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}

	measurement := []string{}
	if config.UsageIdle {
		measurement = append(measurement, "cpu_usage_idle")
	}
	if config.UsageIOWait {
		measurement = append(measurement, "cpu_usage_iowait")
	}
	if config.UsageSteal {
		measurement = append(measurement, "cpu_usage_steal")
	}
	if config.UsageGuest {
		measurement = append(measurement, "cpu_usage_guest")
	}
	if config.UsageUser {
		measurement = append(measurement, "cpu_usage_user")
	}
	if config.UsageSystem {
		measurement = append(measurement, "cpu_usage_system")
	}
	resultMap[util.MapKeyMeasurement] = measurement

	return "cpu", resultMap
}

func (config *CPU) Enable() {
	config.PerCore = true
	config.TotalCPU = true

	config.UsageIdle = true
	config.UsageIOWait = true
	config.UsageSteal = true
	config.UsageGuest = true
	config.UsageUser = true
	config.UsageSystem = true
}
