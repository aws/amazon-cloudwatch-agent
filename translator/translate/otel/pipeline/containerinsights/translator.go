package containerinsights

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
)

const (
	pipelineName = "containerinsights"
)

type translator struct {
}

var _ common.Translator[common.Pipeline] = (*translator)(nil)

func NewTranslator() common.Translator[common.Pipeline] {
	return &translator{}
}

// Type is not used.
func (t *translator) Type() config.Type {
	return pipelineName
}

// Translate creates a pipeline for container insights if the logs.metrics_collected.ecs
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (common.Pipeline, error) {
	key := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.ECSKey)
	if conf != nil && conf.IsSet(key) {
		id := config.NewComponentIDWithName(config.MetricsDataType, pipelineName)
		pipeline := &service.ConfigServicePipeline{
			Receivers:  []config.ComponentID{config.NewComponentID("awscontainerinsightreceiver")},
			Processors: []config.ComponentID{config.NewComponentIDWithName("batch", pipelineName)},
			Exporters:  []config.ComponentID{config.NewComponentIDWithName("awsemf", pipelineName)},
		}
		return util.NewPair(id, pipeline), nil
	}
	return nil, nil
}
