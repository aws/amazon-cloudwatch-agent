package processor

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type translator struct {
	factory component.ProcessorFactory
}

func NewDefaultTranslator(factory component.ProcessorFactory) common.Translator[config.Processor] {
	return &translator{factory}
}

func (t *translator) Translate(*confmap.Conf) (config.Processor, error) {
	return t.factory.CreateDefaultConfig(), nil
}

func (t *translator) Type() config.Type {
	return t.factory.Type()
}
