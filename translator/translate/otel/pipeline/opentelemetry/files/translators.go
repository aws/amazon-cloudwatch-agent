// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	globallogs "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	logsutil "github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
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

	// timestampFormatMagicValue in multi_line_start_pattern means "use the regex
	// generated from timestamp_format as the multiline pattern".
	timestampFormatMagicValue = "{timestamp_format}"
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
	for _, item := range list {
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

		logGroupName, _ := m[logGroupNameKey].(string)
		if logGroupName != "" {
			logGroupName = logsutil.ResolvePlaceholder(logGroupName, globallogs.GlobalLogConfig.MetadataInfo)
		}
		logStreamName, _ := m[logStreamNameKey].(string)
		if logStreamName != "" {
			logStreamName = logsutil.ResolvePlaceholder(logStreamName, globallogs.GlobalLogConfig.MetadataInfo)
		}

		resource := map[string]string{
			"aws.log.source": common.FilesKey,
		}

		entries = append(entries, fileEntry{
			index:            len(entries),
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
