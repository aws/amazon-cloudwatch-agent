package emfprocessor

const (
	SectionKeyMetricUnit = "metric_unit"
)

type MetricUnit struct {
}

func (mu *MetricUnit) ApplyRule(input interface{}) (string, interface{}) {
	im := input.(map[string]interface{})

	if val, ok := im[SectionKeyMetricUnit]; !ok {
		return "", nil
	} else {
		return SectionKeyMetricUnit, val
	}
}

func init() {
	RegisterRule(SectionKeyMetricUnit, new(MetricUnit))
}