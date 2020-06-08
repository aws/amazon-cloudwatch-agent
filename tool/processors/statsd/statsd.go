package statsd

import (
	"strconv"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric/statsd"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/collectd"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	yes := util.Yes("Do you want to turn on StatsD daemon?")
	if yes {
		collection := config.MetricsConf().Collection()
		collection.StatsD = new(statsd.StatsD)
		whichPort(collection.StatsD)
		whichMetricsCollectionInterval(collection.StatsD)
		whichMetricsAggregationInterval(collection.StatsD)
	}
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return collectd.Processor
}

func whichPort(config *statsd.StatsD) {
	answer := util.AskWithDefault("Which port do you want StatsD daemon to listen to?", "8125")
	answer = ":" + answer
	config.ServiceAddress = answer
}

func whichMetricsCollectionInterval(config *statsd.StatsD) {
	answer := util.Choice("What is the collect interval for StatsD daemon?", 1, []string{"10s", "30s", "60s"})
	config.MetricsCollectionInterval, _ = strconv.Atoi(answer[:2])
}

func whichMetricsAggregationInterval(config *statsd.StatsD) {
	answer := util.Choice("What is the aggregation interval for metrics collected by StatsD daemon?",
		4, []string{"Do not aggregate", "10s", "30s", "60s"})
	if answer != "Do not aggregate" {
		config.MetricsAggregationInterval, _ = strconv.Atoi(answer[:2])
	}
}
