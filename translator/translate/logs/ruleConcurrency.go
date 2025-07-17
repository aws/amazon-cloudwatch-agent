// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"errors"
	"runtime"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/constants"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

const (
	ConcurrencySectionKey = "concurrency"
	disableConcurrency    = -1
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
	concurrency := getConcurrency(input)
	if concurrency > 1 {
		result[ConcurrencySectionKey] = concurrency
		GlobalLogConfig.Concurrency = concurrency
	} else {
		GlobalLogConfig.Concurrency = disableConcurrency
	}
	return Output_Cloudwatch_Logs, result
}

func getConcurrency(input any) int {
	m, ok := input.(map[string]any)
	if !ok {
		return disableConcurrency
	}
	v, ok := m[ConcurrencySectionKey].(float64)
	if ok {
		return int(v)
	}
	if _, ok = m[constants.SectionKeyLogsCollected]; !ok {
		return disableConcurrency
	}
	return determineDefault(m)
}

// determineDefault determines the default concurrency if not set. Will not set a default if timestamp_format is
// missing in the configuration for the files being collected.
func determineDefault(input any) int {
	if isMissingAnyTimestampFormat(input, logFileCollectListPath) {
		return disableConcurrency
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
