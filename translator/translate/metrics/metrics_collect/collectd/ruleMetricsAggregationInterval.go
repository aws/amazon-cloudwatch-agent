package collected

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

type MetricsAggregationInterval struct {
}

const SectionKey_MetricsAggregationInterval = "metrics_aggregation_interval"

func (obj *MetricsAggregationInterval) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	return util.ProcessMetricsAggregationInterval(input, "60s", SectionKey)
}

func init() {
	obj := new(MetricsAggregationInterval)
	RegisterRule(SectionKey_MetricsAggregationInterval, obj)
}
