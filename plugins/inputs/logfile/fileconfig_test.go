// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileConfigInit(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
	}

	err := fileConfig.init()
	assert.NoError(t, err)

	assert.Equal(t, time.UTC, fileConfig.TimezoneLoc, "The timezone location should be in UTC.")

	assert.NotNil(t, fileConfig.TimestampRegexP, "The timestampFromLogLine regex pattern should not be nil.")
	assert.Equal(t, fileConfig.TimestampRegex, fileConfig.TimestampRegexP.String(),
		fmt.Sprintf("The compiled timestampFromLogLine regex pattern %v does not align with the given timestampFromLogLine regex string %v.",
			fileConfig.TimestampRegexP.String(),
			fileConfig.TimestampRegex))

	assert.NotNil(t, fileConfig.MultiLineStartPatternP, "The multiline start pattern should not be nil.")
	assert.True(t, fileConfig.MultiLineStartPatternP == fileConfig.TimestampRegexP, "The multiline start pattern should be the same as the timestampFromLogLine pattern.")

	assert.Equal(t, time.UTC, fileConfig.TimezoneLoc, "The timezone location should be UTC.")
}

func TestFileConfigInitFailureCase(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}+)",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
	}

	err := fileConfig.init()
	assert.Error(t, err)
	assert.Equal(t, "timestamp_regex has issue, regexp: Compile( (\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}+) ): error parsing regexp: invalid nested repetition operator: `{2}+`", err.Error())

	fileConfig = &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}+)",
	}

	err = fileConfig.init()
	assert.Error(t, err)
	assert.Equal(t, "multi_line_start_pattern has issue, regexp: Compile( (\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}+) ): error parsing regexp: invalid nested repetition operator: `{2}+`", err.Error())
}

func TestLogGroupName(t *testing.T) {
	filepath := "/tmp/logfile.log.2017-06-19-13"
	expectLogGroup := "/tmp/logfile.log"
	logGroupName := logGroupName(filepath)
	assert.Equal(t, expectLogGroup, logGroupName, fmt.Sprintf(
		"The log group name %s is not the same as %s.",
		logGroupName,
		expectLogGroup))
}

func TestTimestampParser(t *testing.T) {
	timestampRegex := "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})"
	timestampLayout := "02 Jan 2006 15:04:05"
	timezone := "UTC"
	timezoneLoc := time.UTC
	timestampRegexP, err := regexp.Compile(timestampRegex)
	require.NoError(t, err, fmt.Sprintf("Failed to compile regex %s", timestampRegex))
	fileConfig := &FileConfig{
		TimestampRegex:  timestampRegex,
		TimestampRegexP: timestampRegexP,
		TimestampLayout: timestampLayout,
		Timezone:        timezone,
		TimezoneLoc:     timezoneLoc}

	expectedTimestamp := time.Unix(1497882318, 0)
	timestampString := "19 Jun 2017 14:25:18"
	logEntry := fmt.Sprintf("%s [INFO] This is a test message.", timestampString)
	timestamp := fileConfig.timestampFromLogLine(logEntry)
	assert.Equal(t, expectedTimestamp.UnixNano(), timestamp.UnixNano(),
		fmt.Sprintf("The timestampFromLogLine value %v is not the same as expected %v.", timestamp, expectedTimestamp))

	// Test regex match for multiline, the first timestamp in multiline should be matched
	logEntry = fmt.Sprintf("%s [INFO] This is the first line.\n19 Jun 2017 14:25:19 [INFO] This is the second line.\n", timestampString)
	timestamp = fileConfig.timestampFromLogLine(logEntry)
	assert.Equal(t, expectedTimestamp.UnixNano(), timestamp.UnixNano(),
		fmt.Sprintf("The timestampFromLogLine value %v is not the same as expected %v.", timestamp, expectedTimestamp))
}

func TestTimestampParserWithFracSeconds(t *testing.T) {
	timestampRegex := "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2},(\\d{1,9}) \\w{3})"
	timestampLayout := "02 Jan 2006 15:04:05,.000 MST"
	timezone := "UTC"
	timezoneLoc := time.UTC
	timestampRegexP, err := regexp.Compile(timestampRegex)
	require.NoError(t, err, fmt.Sprintf("Failed to compile regex %s", timestampRegex))
	fileConfig := &FileConfig{
		TimestampRegex:  timestampRegex,
		TimestampRegexP: timestampRegexP,
		TimestampLayout: timestampLayout,
		Timezone:        timezone,
		TimezoneLoc:     timezoneLoc}

	expectedTimestamp := time.Unix(1497882318, 234000000)
	timestampString := "19 Jun 2017 14:25:18,234088 UTC"
	logEntry := fmt.Sprintf("%s [INFO] This is a test message.", timestampString)
	timestamp := fileConfig.timestampFromLogLine(logEntry)
	assert.Equal(t, expectedTimestamp.UnixNano(), timestamp.UnixNano(),
		fmt.Sprintf("The timestampFromLogLine value %v is not the same as expected %v.", timestamp, expectedTimestamp))

	// Test regex match for multiline, the first timestamp in multiline should be matched
	logEntry = fmt.Sprintf("%s [INFO] This is the first line.\n19 Jun 2017 14:25:19,123456 UTC [INFO] This is the second line.\n", timestampString)
	timestamp = fileConfig.timestampFromLogLine(logEntry)
	assert.Equal(t, expectedTimestamp.UnixNano(), timestamp.UnixNano(),
		fmt.Sprintf("The timestampFromLogLine value %v is not the same as expected %v.", timestamp, expectedTimestamp))
}

func TestMultiLineStartPattern(t *testing.T) {
	multiLineStartPattern := "---"
	fileConfig := &FileConfig{
		MultiLineStartPattern:  multiLineStartPattern,
		MultiLineStartPatternP: regexp.MustCompile(multiLineStartPattern)}

	logEntryLine := "--------------------"
	multiLineStart := fileConfig.isMultilineStart(logEntryLine)
	assert.True(t, multiLineStart, "This should be a multi-line start line.")

	logEntryLine = "XXXXXXX"
	multiLineStart = fileConfig.isMultilineStart(logEntryLine)
	assert.False(t, multiLineStart, "This should not be a multi-line start line.")
}
