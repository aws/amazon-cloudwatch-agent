// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windowsevents

import (
	"fmt"
	"log"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	collectListKey   = "collect_list"
	eventNameKey     = "event_name"
	eventLevelsKey   = "event_levels"
	eventIDsKey      = "event_ids"
	eventFormatKey   = "event_format"
	logGroupNameKey  = "log_group_name"
	logStreamNameKey = "log_stream_name"
)

var configKey = common.WindowsEventsConfigKey

func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	if conf == nil || !conf.IsSet(configKey) {
		return translators
	}
	if translatorcontext.CurrentContext().Os() != translatorconfig.OS_TYPE_WINDOWS {
		log.Printf("E! windows_events is only supported on Windows, ignoring on %s", translatorcontext.CurrentContext().Os())
		return translators
	}
	for _, entry := range parseEntries(conf) {
		translators.Set(&windowsEventsPipelineTranslator{entry: entry})
	}
	return translators
}

func parseEntries(conf *confmap.Conf) []eventEntry {
	key := common.ConfigKey(configKey, collectListKey)
	val := conf.Get(key)
	list, ok := val.([]any)
	if !ok || len(list) == 0 {
		return nil
	}

	var entries []eventEntry
	var idx int
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		channel, _ := m[eventNameKey].(string)
		if channel == "" {
			continue
		}

		format, _ := m[eventFormatKey].(string)
		raw := format == "xml"

		resource := map[string]string{
			"aws.log.source":  common.WindowsEventsKey,
			"aws.log.channel": channel,
		}
		if logGroup, ok := m[logGroupNameKey].(string); ok && logGroup != "" {
			resource["aws.log.group.name"] = logGroup
		}
		if logStream, ok := m[logStreamNameKey].(string); ok && logStream != "" {
			resource["aws.log.stream.name"] = logStream
		}

		var levels []string
		if rawLevels, ok := m[eventLevelsKey].([]any); ok {
			for _, l := range rawLevels {
				if s, ok := l.(string); ok {
					levels = append(levels, s)
				}
			}
		}

		var ids []int
		if rawIDs, ok := m[eventIDsKey].([]any); ok {
			for _, id := range rawIDs {
				switch v := id.(type) {
				case float64:
					ids = append(ids, int(v))
				case int:
					ids = append(ids, v)
				}
			}
		}

		entries = append(entries, eventEntry{
			name:        fmt.Sprintf("%s_%d", strings.ToLower(channel), idx),
			channel:     channel,
			raw:         raw,
			resource:    resource,
			eventLevels: levels,
			eventIDs:    ids,
		})
		idx++
	}
	return entries
}
