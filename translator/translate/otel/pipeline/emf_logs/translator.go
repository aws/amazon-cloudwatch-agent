// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emf_logs

import (
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"
	"strings"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

const (
	PipelineName = "emf_logs"
)

var (
	key               = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.Emf)
	serviceAddressKey = common.ConfigKey(key, common.ServiceAddress)
)

type translator struct {
}

var _ common.Translator[common.Pipeline] = (*translator)(nil)

func NewTranslator() common.Translator[common.Pipeline] {
	return &translator{}
}

func (t *translator) Type() component.Type {
	return PipelineName
}

// Translate creates a pipeline for emf if emf logs are collected
// section is present.
func (t *translator) Translate(conf *confmap.Conf, _ common.TranslatorOptions) (common.Pipeline, error) {
	if conf == nil || (!conf.IsSet(key)) {
		return nil, &common.MissingKeyError{Type: t.Type(), JsonKey: key}
	}
	id := component.NewIDWithName(component.DataTypeLogs, PipelineName)
	var com []component.ID
	if conf.IsSet(serviceAddressKey) {
		serviceAddress := fmt.Sprintf("%v", conf.Get(serviceAddressKey))
		if strings.Contains(serviceAddress, common.Udp) {
			com = []component.ID{component.NewIDWithName("udplog", PipelineName)}
		} else {
			com = []component.ID{component.NewIDWithName("tcplog", PipelineName)}
		}

	} else {
		com = []component.ID{component.NewIDWithName("udplog", PipelineName), component.NewIDWithName("tcplog", PipelineName)}
	}
	pipeline := &service.ConfigServicePipeline{
		Receivers:  com,
		Processors: []component.ID{component.NewIDWithName("batch", PipelineName)},
		Exporters:  []component.ID{component.NewIDWithName("awscloudwatchlogs", PipelineName)},
	}
	return collections.NewPair(id, pipeline), nil
}
