package containerinsights

import (
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
)

func NewTranslators(conf *confmap.Conf) (pipeline.TranslatorMap, error) {
	translators := common.NewTranslatorMap[*common.ComponentTranslators]()
	// create default container insights translator
	ciTranslator := NewTranslatorWithName(ciPipelineName)
	// create kueue container insights translator
	kueueTranslator := NewTranslatorWithName(kueuePipelineName)
	// add both to the translator map
	translators.Set(ciTranslator)
	translators.Set(kueueTranslator)
	// return the translator map
	return translators, nil
}
