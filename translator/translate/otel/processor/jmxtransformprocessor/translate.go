// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxtransformprocessor

import (
	_ "embed"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed testdata/config.yaml
var transformJmxConfig string

type translator struct {
	name    string
	factory processor.Factory
}
type Context struct {
	name       string
	statements []string
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, transformprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if !(conf != nil && conf.IsSet(common.ContainerInsightsConfigKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.ContainerInsightsConfigKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*transformprocessor.Config)
	clusterName := conf.Get(common.ConfigKey(common.ContainerInsightsConfigKey, "cluster_name"))

	if clusterName == nil {
		return common.GetYamlFileToYamlConfig(cfg, transformJmxConfig)
	}
	c := confmap.NewFromStringMap(map[string]interface{}{
		"metric_statements": []map[string]interface{}{
			{
				"context": "resource",
				"statements": []string{
					"keep_keys(attributes, [\"ClusterName\", \"Namespace\",\"NodeName\"])",
				},
			},
			{
				"context": "metric",
				"statements": []string{
					"set(unit, \"Bytes\") where name == \"jvm.memory.heap.used\"",
					"set(unit, \"Bytes\") where name == \"jvm.memory.nonheap.used\"",
					"set(unit, \"Bytes\") where name == \"jvm.memory.pool.used\"",
					"set(unit, \"Bytes\") where name == \"tomcat.traffic.received\"",
					"set(unit, \"Bytes\") where name == \"tomcat.traffic.sent\"",
					"set(unit, \"Bytes\") where name == \"jvm.operating.system.total.swap.space.size\"",
					"set(unit, \"Bytes\") where name == \"jvm.operating.system.free.swap.space.size\"",
					"set(unit, \"Bytes\") where name == \"jvm.operating.system.total.physical.memory.size\"",
					"set(unit, \"Bytes\") where name == \"jvm.operating.system.free.physical.memory.size\"",
					"set(unit, \"Count\") where name == \"tomcat.sessions\"",
					"set(unit, \"Count\") where name == \"tomcat.rejected_sessions\"",
					"set(unit, \"Count\") where name == \"jvm.threads.count\"",
					"set(unit, \"Count\") where name == \"jvm.threads.daemon\"",
					"set(unit, \"Count\") where name == \"jvm.operating.system.open.file.descriptor.count\"",
					"set(unit, \"Count\") where name == \"jvm.operating.system.available.processors\"",
					"set(unit, \"Count\") where name == \"tomcat.request_count\"",
					"set(unit, \"Count\") where name == \"tomcat.errors\"",
					"set(unit, \"Count\") where name == \"jvm.classes.loaded\"",
					"set(unit, \"Count\") where name == \"jvm.operating.system.system.cpu.load\"",
					"set(unit, \"Count\") where name == \"jvm.operating.system.process.cpu.load\"",
					"set(unit, \"Milliseconds\") where name == \"tomcat.processing_time\"",
				},
			},
		},
	})

	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal jmx filter processor (%s): %w", t.ID(), err)
	}

	return cfg, nil
}
