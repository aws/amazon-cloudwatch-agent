// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/profiler"
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

	assert.Nil(t, fileConfig.Filters)
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

func TestTimestampParserWithPadding(t *testing.T) {
	timestampRegex := "(\\d{1,2} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})"
	timestampLayout := "1 2 15:04:05"
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

	logEntry := fmt.Sprintf(" 2 1 07:10:06 instance-id: i-02fce21a425a2efb3")
	timestamp := fileConfig.timestampFromLogLine(logEntry)
	assert.Equal(t, 7, timestamp.Hour(), fmt.Sprintf("Timestamp does not match: %v, act: %v", "7", timestamp.Hour()))
	assert.Equal(t, 10, timestamp.Minute(), fmt.Sprintf("Timestamp does not match: %v, act: %v", "10", timestamp.Minute()))

	logEntry = fmt.Sprintf("2 1 07:10:06 instance-id: i-02fce21a425a2efb3")
	timestamp = fileConfig.timestampFromLogLine(logEntry)
	assert.Equal(t, 7, timestamp.Hour(), fmt.Sprintf("Timestamp does not match: %v, act: %v", "7", timestamp.Hour()))
	assert.Equal(t, 10, timestamp.Minute(), fmt.Sprintf("Timestamp does not match: %v, act: %v", "10", timestamp.Minute()))
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

func TestFileConfigInitWithFilters(t *testing.T) {
	filter1 := LogFilter{
		Type:       includeType,
		Expression: "StatusCode: [4-5]\\d\\d",
	}
	filter2 := LogFilter{
		Type:       excludeType,
		Expression: "Some expression that (will|won't) compile",
	}

	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters:               []LogFilter{filter1, filter2},
	}

	err := fileConfig.init()
	assert.NoError(t, err)

	assert.Len(t, fileConfig.Filters, 2)
	f := fileConfig.Filters[0]
	assert.NotNil(t, f.ExpressionP)
	assert.Equal(t, filter1.Type, f.Type)
	assert.Equal(t, filter1.Expression, f.Expression)
	f = fileConfig.Filters[1]
	assert.NotNil(t, f.ExpressionP)
	assert.Equal(t, filter2.Type, f.Type)
	assert.Equal(t, filter2.Expression, f.Expression)
}

func TestFileConfigInitWithFiltersFails(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       excludeType,
				Expression: "Some expression that (will|won't) compile",
			},
			{
				Type:       includeType,
				Expression: "StatusCode: ([4-5]\\d\\d", // invalid regexp
			},
		},
	}

	err := fileConfig.init()
	assert.Error(t, err)
	assert.Equal(t, "filter regex has issue, regexp: Compile( StatusCode: ([4-5]\\d\\d ): error parsing regexp: missing closing ): `StatusCode: ([4-5]\\d\\d`", err.Error())
}

func TestLogIncludeFilter(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{{
			Type:       includeType,
			Expression: "StatusCode: [4-5]\\d\\d",
		}},
	}

	err := fileConfig.init()
	assert.NoError(t, err)
	res := fileConfig.shouldFilterLog(LogEvent{
		msg: "API responded with [StatusCode: 500] for call to /foo/bar",
	})
	assert.False(t, res)

	res = fileConfig.shouldFilterLog(LogEvent{
		msg: "This is another log message that doesn't match",
	})
	assert.True(t, res)
}

func TestLogExcludeFilter(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{{
			Type:       excludeType,
			Expression: "StatusCode: [4-5]\\d\\d",
		}},
	}

	err := fileConfig.init()
	assert.NoError(t, err)
	res := fileConfig.shouldFilterLog(LogEvent{
		msg: "API responded with [StatusCode: 500] for call to /foo/bar",
	})
	assert.True(t, res)

	res = fileConfig.shouldFilterLog(LogEvent{
		msg: "This is another log message that doesn't match",
	})
	assert.False(t, res)
}

func TestLogIncludeThenExcludeFilter(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       includeType,
				Expression: "search_(\\w+)",
			},
			{
				Type:       excludeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(t, err)
	res := fileConfig.shouldFilterLog(LogEvent{
		msg: "API responded with [StatusCode: 500] for call to /foo/bar",
	})
	assert.True(t, res)
	res = fileConfig.shouldFilterLog(LogEvent{
		msg: "Submitted request to application search_FooBarBaz1",
	})
	assert.False(t, res)
	// If the log message matches both, it should short-circuit and evaluate only the first one
	res = fileConfig.shouldFilterLog(LogEvent{
		msg: "Here is a log for search_Abc123 that also has a status code of (StatusCode: 425) and that's it.",
	})
	assert.False(t, res)
	// If the log message matches neither, because this config has an include expression,
	// drop the log
	res = fileConfig.shouldFilterLog(LogEvent{
		msg: "Some other log that doesn't match either expression",
	})
	assert.True(t, res)
}

func TestLogFilterExclusionsDoNotDropUnmatchedLog(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       excludeType,
				Expression: "search_(\\w+)",
			},
			{
				Type:       excludeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(t, err)

	res := fileConfig.shouldFilterLog(LogEvent{
		msg: "Some other log that doesn't match either expression",
	})
	assert.False(t, res)
}

func TestLogFilterSampleCountResets(t *testing.T) {
	profiler.Profiler.ReportAndClear()
	original := sampleThreshold
	defer func() {
		sampleThreshold = original
	}()
	sampleThreshold = 3 // on the third invocation, it should reset

	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{{
			Type:       excludeType,
			Expression: "StatusCode: [4-5]\\d\\d",
		}},
	}

	err := fileConfig.init()
	assert.NoError(t, err)

	res := fileConfig.shouldFilterLog(LogEvent{
		msg: "API responded with [StatusCode: 500] for call to /foo/bar",
	})
	assert.True(t, res)
	assert.Equal(t, 1, fileConfig.sampleCount)

	res = fileConfig.shouldFilterLog(LogEvent{
		msg: "This is another log message that doesn't match",
	})
	assert.False(t, res)
	assert.Equal(t, 2, fileConfig.sampleCount)

	res = fileConfig.shouldFilterLog(LogEvent{
		msg: "API responded with [StatusCode: 500] for call to /foo/bar",
	})
	assert.True(t, res)
	assert.Equal(t, 0, fileConfig.sampleCount)
}

func TestLogFilterSampleCountResetsIfNotMatched(t *testing.T) {
	profiler.Profiler.ReportAndClear()
	original := sampleThreshold
	defer func() {
		sampleThreshold = original
	}()
	sampleThreshold = 3 // on the third invocation, it should reset

	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       excludeType,
				Expression: "search_(\\w+)",
			},
			{
				Type:       excludeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(t, err)

	res := fileConfig.shouldFilterLog(LogEvent{
		msg: "Some other log that doesn't match either expression",
	})
	assert.False(t, res)
	assert.Equal(t, 1, fileConfig.sampleCount)

	res = fileConfig.shouldFilterLog(LogEvent{
		msg: "Some other log that doesn't match either expression",
	})
	assert.False(t, res)
	assert.Equal(t, 2, fileConfig.sampleCount)

	res = fileConfig.shouldFilterLog(LogEvent{
		msg: "Some other log that doesn't match either expression",
	})
	assert.False(t, res)
	assert.Equal(t, 0, fileConfig.sampleCount)
}

func BenchmarkLogFilterSimpleInclude(b *testing.B) {
	original := sampleThreshold
	defer func() {
		sampleThreshold = original
	}()
	sampleThreshold = b.N * 5
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       includeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(b, err)
	var res bool
	log := LogEvent{
		msg: "API responded with [StatusCode: 409] for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		res = fileConfig.shouldFilterLog(log)
	}
	assert.False(b, res)
}

func BenchmarkLogFilterSimpleIncludeNotMatch(b *testing.B) {
	original := sampleThreshold
	defer func() {
		sampleThreshold = original
	}()
	sampleThreshold = b.N * 5
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       includeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(b, err)
	var res bool
	log := LogEvent{
		msg: "API responded with [StatusCode: 209] for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		res = fileConfig.shouldFilterLog(log)
	}
	assert.True(b, res)
}

func BenchmarkLogFilterSimpleExclude(b *testing.B) {
	original := sampleThreshold
	defer func() {
		sampleThreshold = original
	}()
	sampleThreshold = b.N * 5
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       excludeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(b, err)
	var res bool
	log := LogEvent{
		msg: "API responded with [StatusCode: 409] for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		res = fileConfig.shouldFilterLog(log)
	}
	assert.True(b, res)
}

func BenchmarkLogFilterSimpleExcludeNotMatch(b *testing.B) {
	original := sampleThreshold
	defer func() {
		sampleThreshold = original
	}()
	sampleThreshold = b.N * 5
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       excludeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(b, err)
	var res bool
	log := LogEvent{
		msg: "API responded with [StatusCode: 209] for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		res = fileConfig.shouldFilterLog(log)
	}
	assert.False(b, res)
}

func BenchmarkLogFilterIncludeThenMatchExclude(b *testing.B) {
	original := sampleThreshold
	defer func() {
		sampleThreshold = original
	}()
	sampleThreshold = b.N * 5
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       includeType,
				Expression: "search_(\\w+)",
			},
			{
				Type:       excludeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(b, err)
	var res bool
	event := LogEvent{
		msg: "API responded with [StatusCode: 409] for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		res = fileConfig.shouldFilterLog(event)
	}
	assert.True(b, res)
}

func BenchmarkLogFilterExclusionsDoNotDropUnmatchedLog(b *testing.B) {
	original := sampleThreshold
	defer func() {
		sampleThreshold = original
	}()
	sampleThreshold = b.N * 5
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       excludeType,
				Expression: "search_(\\w+)",
			},
			{
				Type:       excludeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(b, err)
	var res bool
	event := LogEvent{
		msg: "Some other log that doesn't match either expression",
	}
	for i := 0; i < b.N; i++ {
		res = fileConfig.shouldFilterLog(event)
	}
	assert.False(b, res)
}

func BenchmarkLogFilterMatchLastFilter(b *testing.B) {
	original := sampleThreshold
	defer func() {
		sampleThreshold = original
	}()
	sampleThreshold = b.N * 5
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       includeType,
				Expression: "(ERROR|WARN)",
			},
			{
				Type:       excludeType,
				Expression: "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
			},
			{
				Type:       includeType,
				Expression: "search_(\\w+)",
			},
			{
				Type:       excludeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
			{
				Type:       includeType,
				Expression: "/(\\w+)/(\\w+)/amazon-cloudwatch-agent",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(b, err)
	var res bool
	event := LogEvent{
		msg: "2000/01/01 02:55:39 I! Config has been translated into TOML /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.toml",
	}
	for i := 0; i < b.N; i++ {
		res = fileConfig.shouldFilterLog(event)
	}
	assert.False(b, res)
}

func BenchmarkLogFilterMatchFirstFilter(b *testing.B) {
	original := sampleThreshold
	defer func() {
		sampleThreshold = original
	}()
	sampleThreshold = b.N * 5
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       "02 Jan 2006 15:04:05",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		Filters: []LogFilter{
			{
				Type:       includeType,
				Expression: "(ERROR|WARN)",
			},
			{
				Type:       excludeType,
				Expression: "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
			},
			{
				Type:       includeType,
				Expression: "search_(\\w+)",
			},
			{
				Type:       excludeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
			{
				Type:       includeType,
				Expression: "/(\\w+)/(\\w+)/amazon-cloudwatch-agent",
			},
		},
	}

	err := fileConfig.init()
	assert.NoError(b, err)
	var res bool
	event := LogEvent{
		msg: "2000/01/01 02:55:39 ERROR: Config has been translated into TOML /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.toml",
	}
	for i := 0; i < b.N; i++ {
		res = fileConfig.shouldFilterLog(event)
	}
	assert.False(b, res)
}
