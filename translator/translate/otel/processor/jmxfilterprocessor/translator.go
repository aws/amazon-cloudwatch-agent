// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxfilterprocessor

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	matchTypeStrict = "strict"
)

type translator struct {
	name    string
	factory processor.Factory
}

type Option func(any)

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name: name, factory: filterprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(common.ContainerInsightsConfigKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.ContainerInsightsConfigKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)

	includeMetricNames := []string{
		"jvm.classes.loaded",
		"jvm.memory.heap.used",
		"jvm.memory.nonheap.used",
		"jvm.memory.pool.used",
		"jvm.operating.system.total.swap.space.size",
		"jvm.operating.system.system.cpu.load",
		"jvm.operating.system.process.cpu.load",
		"jvm.operating.system.free.swap.space.size",
		"jvm.operating.system.total.physical.memory.size",
		"jvm.operating.system.free.physical.memory.size",
		"jvm.operating.system.open.file.descriptor.count",
		"jvm.operating.system.available.processors",
		"jvm.threads.count",
		"jvm.threads.daemon",
		"tomcat.sessions",
		"tomcat.rejected_sessions",
		"tomcat.traffic.received",
		"tomcat.traffic.sent",
		"tomcat.request_count",
		"tomcat.errors",
		"tomcat.processing_time",
	}

	c := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]any{
			"include": map[string]any{
				"match_type":   matchTypeStrict,
				"metric_names": includeMetricNames,
			},
		},
	})

	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal jmx filter processor (%s): %w", t.ID(), err)
	}

	return cfg, nil
}
