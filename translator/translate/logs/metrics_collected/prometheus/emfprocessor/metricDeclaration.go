package emfprocessor

const (
	SectionKeyMetricDeclaration = "metric_declaration"
)

type MetricDeclaration struct {
}

func (m *MetricDeclaration) ApplyRule(input interface{}) (string, interface{}) {
	im := input.(map[string]interface{})

	if val, ok := im[SectionKeyMetricDeclaration]; !ok {
		return "", nil
	} else {
		return SectionKeyMetricDeclaration, val
	}
}

func init() {
	RegisterRule(SectionKeyMetricDeclaration, new(MetricDeclaration))
}
