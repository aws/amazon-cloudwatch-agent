// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/cloudwatch"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/adapter"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
)

const (
	pipelineName = "host"
)

type translator struct {
}

var _ common.Translator[common.Pipeline] = (*translator)(nil)

func NewTranslator() common.Translator[common.Pipeline] {
	return &translator{}
}

// Type is not used.
func (t translator) Type() config.Type {
	return pipelineName
}

// Translate creates a pipeline if metrics section exists.
func (t translator) Translate(conf *confmap.Conf) (common.Pipeline, error) {
	if conf != nil && conf.IsSet(common.MetricsKey) {
		id := config.NewComponentIDWithName(config.MetricsDataType, pipelineName)
		pipeline := &service.ConfigServicePipeline{
			Receivers:  []config.ComponentID{config.NewComponentID(adapter.Type("cpu"))},
			Processors: []config.ComponentID{config.NewComponentIDWithName("cumulativetodelta", pipelineName)},
			Exporters:  []config.ComponentID{config.NewComponentIDWithName(cloudwatch.TypeStr, pipelineName)},
		}
		return util.NewPair(id, pipeline), nil
	}
	return nil, nil
}
