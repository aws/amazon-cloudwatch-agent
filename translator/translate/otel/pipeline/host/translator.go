// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"log"
	"sort"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/cloudwatch"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/processors/ec2tagger"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type translator struct {
	receivers    []component.ID
	pipelineName component.Type
}

var _ common.Translator[common.Pipeline] = (*translator)(nil)

// NewTranslator creates a new host pipeline translator. The receiver types
// passed in are converted to config.ComponentIDs, sorted, and used directly
// in the translated pipeline.
func NewTranslator(receiverTypes []component.Type, pipelineName component.Type) common.Translator[common.Pipeline] {
	receivers := make([]component.ID, len(receiverTypes))
	for i, receiver := range receiverTypes {
		receivers[i] = component.NewID(receiver)
	}
	sort.Slice(receivers, func(i, j int) bool {
		return receivers[i].String() < receivers[j].String()
	})
	return &translator{receivers, pipelineName}
}

func (t translator) Type() component.Type {
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
	var processors []component.ID
	if common.HostDeltaMetricsPipelineName == t.pipelineName {
		log.Printf("D! delta processor required because metrics with diskio or net are required")
		processors = append(processors, component.NewIDWithName("cumulativetodelta", string(t.pipelineName)))
	}

	key := common.ConfigKey(common.MetricsKey, "append_dimensions")
	if conf.IsSet(key) {
		log.Printf("D! ec2tagger processor required because append_dimensions is set")
		processors = append(processors, component.NewID(ec2tagger.TypeStr))
	}
	id := component.NewIDWithName(component.DataTypeMetrics, string(t.pipelineName))
	pipeline := &service.ConfigServicePipeline{
		Receivers:  t.receivers,
		Processors: processors,
		Exporters:  []component.ID{component.NewID(cloudwatch.TypeStr)},
	}
	return collections.NewPair(id, pipeline), nil
}
