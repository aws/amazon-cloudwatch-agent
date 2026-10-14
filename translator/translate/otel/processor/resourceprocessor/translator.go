// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourceprocessor

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
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

type Action string

const (
	ActionInsert Action = "insert"
	ActionUpdate Action = "update"
	ActionUpsert Action = "upsert"
)

type AttributeAction struct {
	Action        Action
	Key           string
	Value         string
	FromAttribute string
}

func WithOrderedActions(actions []AttributeAction) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.orderedActions = actions
		}
	}
}

// WithReservedKeys rejects the given attribute keys in the static-attributes
// path so customer-supplied resource_attributes cannot clobber attributes the
// agent manages internally (e.g. log routing keys).
func WithReservedKeys(keys ...string) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.reservedKeys = collections.NewSet(keys...)
		}
	}
}

type translator struct {
	common.NameProvider
	common.IndexProvider
	factory        processor.Factory
	attributes     map[string]string
	orderedActions []AttributeAction
	reservedKeys   collections.Set[string]
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
	if len(t.orderedActions) > 0 {
		return t.translateActions(t.orderedActions)
	}
	if len(t.attributes) > 0 {
		return t.translateActions(t.staticAttributeActions())
	}
	return t.translateJMX(conf)
}

func (t *translator) staticAttributeActions() []AttributeAction {
	keys := make([]string, 0, len(t.attributes))
	for k := range t.attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	actions := make([]AttributeAction, 0, len(keys))
	for _, k := range keys {
		actions = append(actions, AttributeAction{Action: ActionUpsert, Key: k, Value: t.attributes[k]})
	}
	return actions
}

func (t *translator) translateActions(actions []AttributeAction) (component.Config, error) {
	var errs error
	attrs := make([]any, 0, len(actions))
	for _, action := range actions {
		if strings.TrimSpace(action.Key) == "" {
			errs = errors.Join(errs, fmt.Errorf("%s: resource attribute keys must not be empty", t.ID()))
			continue
		}
		if t.reservedKeys.Contains(action.Key) {
			errs = errors.Join(errs, fmt.Errorf("%s: resource attribute key %q is reserved and cannot be overridden", t.ID(), action.Key))
			continue
		}
		switch action.Action {
		case ActionInsert, ActionUpdate, ActionUpsert:
		default:
			errs = errors.Join(errs, fmt.Errorf("%s: unsupported resource attributes action %q", t.ID(), action.Action))
			continue
		}
		if action.Value != "" && action.FromAttribute != "" {
			errs = errors.Join(errs, fmt.Errorf("%s: resource attribute %q cannot set both value and from_attribute", t.ID(), action.Key))
			continue
		}
		attr := map[string]any{"action": string(action.Action), "key": action.Key}
		if action.FromAttribute != "" {
			attr["from_attribute"] = action.FromAttribute
		} else {
			attr["value"] = action.Value
		}
		attrs = append(attrs, attr)
	}
	if errs != nil {
		return nil, errs
	}
	cfg := t.factory.CreateDefaultConfig().(*resourceprocessor.Config)
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
	clusterName := common.GetClusterName(conf, common.LegacyClusterNameKey)
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
