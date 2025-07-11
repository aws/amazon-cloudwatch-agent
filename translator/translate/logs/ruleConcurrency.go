// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"errors"
	"runtime"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/constants"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

const (
	ConcurrencySectionKey = "concurrency"
)

var (
	logFileCollectListPath      = util.Path(constants.SectionKeyLogsCollected, constants.SectionKeyFiles, constants.SectionKeyCollectList)
	checkTimestampFormatVisitor = util.NewSliceVisitor(util.NewVisitor(isMissingTimestampFormat))
	// based on AWS Java SDK default
	defaultConcurrency = max(runtime.NumCPU(), 8)
)

type Concurrency struct {
}

func (c *Concurrency) ApplyRule(input any) (string, any) {
	result := map[string]any{}
	_, val := translator.DefaultCase(ConcurrencySectionKey, nil, input)
	var concurrency int
	if v, ok := val.(float64); ok {
		concurrency = int(v)
	} else {
		concurrency = determineDefault(input)
	}
	if concurrency > 1 {
		result[ConcurrencySectionKey] = concurrency
		GlobalLogConfig.Concurrency = concurrency
	} else {
		GlobalLogConfig.Concurrency = -1
	}
	return Output_Cloudwatch_Logs, result
}

// determineDefault determines the default concurrency if not set. Will not set a default if timestamp_format is
// missing in the configuration for the files being collected.
func determineDefault(input any) int {
	m, ok := input.(map[string]any)
	if !ok {
		return -1
	}
	_, ok = m[constants.SectionKeyLogsCollected]
	if !ok || isMissingAnyTimestampFormat(input, logFileCollectListPath) {
		return -1
	}
	return defaultConcurrency
}

func isMissingAnyTimestampFormat(input any, path string) bool {
	return errors.Is(util.Visit(input, path, checkTimestampFormatVisitor), util.ErrTargetNotFound)
}

func isMissingTimestampFormat(input any) error {
	m, ok := input.(map[string]any)
	if !ok {
		return util.ErrTargetNotFound
	}
	filePath, ok := m[constants.SectionKeyFilePath]
	// skip the agent log file if configured as timestamp format is not supported https://github.com/aws/amazon-cloudwatch-agent/pull/885
	if ok && filePath == context.CurrentContext().GetAgentLogFile() {
		return nil
	}
	_, ok = m[constants.SectionKeyTimestampFormat]
	if !ok {
		return util.ErrTargetNotFound
	}
	return nil
}

func init() {
	RegisterRule(ConcurrencySectionKey, new(Concurrency))
}
