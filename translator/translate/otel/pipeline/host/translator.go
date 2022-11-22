// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"log"
	"sort"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/cloudwatch"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type translator struct {
	receivers    []config.ComponentID
	pipelineName config.Type
}

var _ common.Translator[common.Pipeline] = (*translator)(nil)

// NewTranslator creates a new host pipeline translator. The receiver types
// passed in are converted to config.ComponentIDs, sorted, and used directly
// in the translated pipeline.
func NewTranslator(receiverTypes []config.Type, pipelineName config.Type) common.Translator[common.Pipeline] {
	receivers := make([]config.ComponentID, len(receiverTypes))
	for i, receiver := range receiverTypes {
		receivers[i] = config.NewComponentID(receiver)
	}
	sort.Slice(receivers, func(i, j int) bool {
		return receivers[i].String() < receivers[j].String()
	})
	return &translator{receivers, pipelineName}
}

func (t translator) Type() config.Type {
	return t.pipelineName
}

// Translate creates a pipeline if metrics section exists.
func (t translator) Translate(conf *confmap.Conf) (common.Pipeline, error) {
	if conf == nil || !conf.IsSet(common.MetricsKey) {
		return nil, &common.MissingKeyError{Type: t.Type(), JsonKey: common.MetricsKey}
	} else if len(t.receivers) == 0 {
		log.Printf("D! pipeline %s has no receivers", t.pipelineName)
		return nil, nil
	}
	// we need to add delta processor because (only) diskio and net input plugins report delta metric
	var processors []config.ComponentID
	if common.HostDeltaMetricsPipelineName == t.pipelineName {
		log.Printf("D! delta processor required because metrics with diskio or net are required")
		processors = append(processors, config.NewComponentIDWithName("cumulativetodelta", string(t.pipelineName)))
	}
	id := config.NewComponentIDWithName(config.MetricsDataType, string(t.pipelineName))
	pipeline := &service.ConfigServicePipeline{
		Receivers:  t.receivers,
		Processors: processors,
		Exporters:  []config.ComponentID{config.NewComponentID(cloudwatch.TypeStr)},
	}
	return collections.NewPair(id, pipeline), nil
}
