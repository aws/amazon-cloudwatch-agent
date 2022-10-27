// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"sort"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/cloudwatch"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

const (
	pipelineName = "host"
)

type translator struct {
	receivers []config.ComponentID
}

var _ common.Translator[common.Pipeline] = (*translator)(nil)

// NewTranslator creates a new host pipeline translator. The receiver types
// passed in are converted to config.ComponentIDs, sorted, and used directly
// in the translated pipeline.
func NewTranslator(receiverTypes []config.Type) common.Translator[common.Pipeline] {
	receivers := make([]config.ComponentID, len(receiverTypes))
	for i, receiver := range receiverTypes {
		receivers[i] = config.NewComponentID(receiver)
	}
	sort.Slice(receivers, func(i, j int) bool {
		return receivers[i].String() < receivers[j].String()
	})
	return &translator{receivers}
}

func (t translator) Type() config.Type {
	return pipelineName
}

// Translate creates a pipeline if metrics section exists.
func (t translator) Translate(conf *confmap.Conf) (common.Pipeline, error) {
	if conf == nil || !conf.IsSet(common.MetricsKey) {
		return nil, &common.MissingKeyError{Type: t.Type(), JsonKey: common.MetricsKey}
	}
	id := config.NewComponentIDWithName(config.MetricsDataType, pipelineName)
	pipeline := &service.ConfigServicePipeline{
		Receivers:  t.receivers,
		Processors: []config.ComponentID{config.NewComponentIDWithName("cumulativetodelta", pipelineName)},
		Exporters:  []config.ComponentID{config.NewComponentIDWithName(cloudwatch.TypeStr, pipelineName)},
	}
	return collections.NewPair(id, pipeline), nil
}
