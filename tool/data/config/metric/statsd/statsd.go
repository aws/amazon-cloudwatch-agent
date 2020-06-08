package statsd

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

type StatsD struct {
	ServiceAddress             string `service_address`
	MetricsCollectionInterval  int    `metrics_collection_interval`
	MetricsAggregationInterval int    `metrics_aggregation_interval`
}

func (config *StatsD) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})

	if config.ServiceAddress != "" {
		resultMap["service_address"] = config.ServiceAddress
	}
	if config.MetricsCollectionInterval != 0 {
		resultMap["metrics_collection_interval"] = config.MetricsCollectionInterval
	}
	resultMap["metrics_aggregation_interval"] = config.MetricsAggregationInterval
	return "statsd", resultMap
}

func (config *StatsD) Enable() {
	config.ServiceAddress = ":8125"
	config.MetricsCollectionInterval = 10
	config.MetricsAggregationInterval = 60
}
