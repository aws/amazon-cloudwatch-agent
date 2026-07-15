// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourceprocessor

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func WithAttributes(attrs map[string]string) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.attributes = attrs
		}
	}
}

type translator struct {
	common.NameProvider
	common.IndexProvider
	factory    processor.Factory
	attributes map[string]string
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: resourceprocessor.NewFactory()}
	t.SetIndex(-1)
	for _, opt := range opts {
		opt(t)
	}
	if t.Index() != -1 {
		t.SetName(t.Name() + "/" + strconv.Itoa(t.Index()))
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if len(t.attributes) > 0 {
		return t.translateStaticAttributes()
	}
	return t.translateJMX(conf)
}

func (t *translator) translateStaticAttributes() (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*resourceprocessor.Config)
	attrs := make([]any, 0, len(t.attributes))
	for k, v := range t.attributes {
		attrs = append(attrs, map[string]any{
			"action": "upsert",
			"key":    k,
			"value":  v,
		})
	}
	c := confmap.NewFromStringMap(map[string]any{"attributes": attrs})
	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal resource processor: %w", err)
	}
	return cfg, nil
}

func (t *translator) translateJMX(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || (!conf.IsSet(common.JmxConfigKey) && t.Name() != common.PipelineNameContainerInsightsJmx) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.JmxConfigKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*resourceprocessor.Config)
	var attributes []any
	if strings.HasPrefix(t.Name(), common.PipelineNameJmx) {
		attributes = t.getJMXAttributes(conf)
	} else if t.Name() == common.PipelineNameContainerInsightsJmx {
		attributes = t.getContainerInsightsJMXAttributes(conf)
	}
	if len(attributes) == 0 {
		baseKey := common.JmxConfigKey
		if t.Index() != -1 {
			baseKey = fmt.Sprintf("%s[%d]", baseKey, t.Index())
		}
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.ConfigKey(baseKey, common.AppendDimensionsKey)}
	}
	c := confmap.NewFromStringMap(map[string]any{
		"attributes": attributes,
	})

	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal resource processor: %w", err)
	}

	return cfg, nil
}

func (t *translator) getJMXAttributes(conf *confmap.Conf) []any {
	if !context.CurrentContext().RunInContainer() {
		return []any{
			map[string]any{
				"action":  "delete",
				"pattern": "telemetry.sdk.*",
			},
			map[string]any{
				"action": "delete",
				"key":    "service.name",
				"value":  "unknown_service:java",
			},
		}
	}
	jmxMap := common.GetIndexedMap(conf, common.JmxConfigKey, t.Index())
	appendDimensions, ok := jmxMap[common.AppendDimensionsKey].(map[string]any)
	if !ok {
		return nil
	}
	var attributes []any
	for key, value := range appendDimensions {
		attributes = append(attributes, map[string]any{
			"action": "upsert",
			"key":    key,
			"value":  value,
		})
	}
	return attributes
}

func (t *translator) getContainerInsightsJMXAttributes(conf *confmap.Conf) []any {
	clusterName := common.GetClusterName(conf)
	nodeName := os.Getenv(config.HOST_NAME)
	return []any{
		map[string]any{
			"key":            "Namespace",
			"from_attribute": "k8s.namespace.name",
			"action":         "insert",
		},
		map[string]any{
			"key":    "ClusterName",
			"value":  clusterName, // Ensure 'clusterName' is defined earlier
			"action": "upsert",
		},
		map[string]any{
			"key":    "NodeName",
			"value":  nodeName,
			"action": "insert",
		},
	}
}
