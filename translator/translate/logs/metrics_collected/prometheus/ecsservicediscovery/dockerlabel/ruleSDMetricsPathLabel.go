package dockerlabel

import "github.com/aws/amazon-cloudwatch-agent/translator"

const (
	SectionKeySDMetricsPathLabel = "sd_metrics_path_label"
)

type SDMetricsPathLabel struct {
}

func (d *SDMetricsPathLabel) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKeySDMetricsPathLabel, "ECS_PROMETHEUS_METRICS_PATH", input)
	return
}

func init() {
	RegisterRule(SectionKeySDMetricsPathLabel, new(SDMetricsPathLabel))
}
