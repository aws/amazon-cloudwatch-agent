// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/timestamp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	collectListKey        = "collect_list"
	filePathKey           = "file_path"
	logGroupNameKey       = "log_group_name"
	logStreamNameKey      = "log_stream_name"
	multiLineStartPattern = "multi_line_start_pattern"
	timestampFormatKey    = "timestamp_format"
	timezoneKey           = "timezone"
	encodingKey           = "encoding"
)

func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	if conf == nil || !conf.IsSet(common.FilesConfigKey) {
		return translators
	}
	for _, entry := range parseEntries(conf) {
		translators.Set(&filesPipelineTranslator{entry: entry})
	}
	return translators
}

func parseEntries(conf *confmap.Conf) []fileEntry {
	key := common.ConfigKey(common.FilesConfigKey, collectListKey)
	val := conf.Get(key)
	list, ok := val.([]any)
	if !ok || len(list) == 0 {
		return nil
	}

	var entries []fileEntry
	for index, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		filePath, _ := m[filePathKey].(string)
		if filePath == "" {
			continue
		}

		encoding, _ := m[encodingKey].(string)
		if encoding == "" {
			encoding = "utf-8"
		}

		multiline, _ := m[multiLineStartPattern].(string)
		timestampFormat, _ := m[timestampFormatKey].(string)
		timezone, _ := m[timezoneKey].(string)

		// Support {timestamp_format} magic value: use the generated timestamp regex as the multiline pattern.
		if multiline == "{timestamp_format}" && timestampFormat != "" {
			multiline = timestamp.BuildRegex(timestampFormat)
		}

		logGroupName, _ := m[logGroupNameKey].(string)
		logStreamName, _ := m[logStreamNameKey].(string)

		resource := map[string]string{
			"aws.log.source": common.FilesKey,
		}

		entries = append(entries, fileEntry{
			index:            index,
			filePath:         filePath,
			encoding:         encoding,
			multilinePattern: multiline,
			timestampFormat:  timestampFormat,
			timezone:         timezone,
			logGroupName:     logGroupName,
			logStreamName:    logStreamName,
			resource:         resource,
		})
	}
	return entries
}
