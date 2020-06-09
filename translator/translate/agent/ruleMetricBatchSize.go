package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type MetricBatchSize struct {
}

func (m *MetricBatchSize) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("metric_batch_size", 1000, input)
	return
}

func init() {
	m := new(MetricBatchSize)
	RegisterRule("metric_batch_size", m)
}
