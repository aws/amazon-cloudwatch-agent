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

	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestFileConfigInit(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2})",
		TimestampLayout:       []string{"02 Jan 2006 15:04:05"},
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		LogGroupClass:         util.StandardLogGroupClass,
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
	assert.Equal(t, util.StandardLogGroupClass, fileConfig.LogGroupClass)

	assert.Nil(t, fileConfig.Filters)
}

func TestFileConfigInitFailureCase(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}+)",
		TimestampLayout:       []string{"02 Jan 2006 15:04:05"},
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
		TimestampLayout:       []string{"02 Jan 2006 15:04:05"},
		Timezone:              "UTC",
		MultiLineStartPattern: "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}+)",
	}

	err = fileConfig.init()
	assert.Error(t, err)
	assert.Equal(t, "multi_line_start_pattern has issue, regexp: Compile( (\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}+) ): error parsing regexp: invalid nested repetition operator: `{2}+`", err.Error())
}

func TestInfrequent_accessAndEmptyLogGroupClassInit(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}+)",
		TimestampLayout:       []string{"02 Jan 2006 15:04:05"},
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
		LogGroupClass:         util.InfrequentAccessLogGroupClass,
	}

	err := fileConfig.init()
	assert.NotNil(t, err)
	assert.Equal(t, util.InfrequentAccessLogGroupClass, fileConfig.LogGroupClass)

	fileConfig = &FileConfig{
		FilePath:              "/tmp/logfile.log",
		LogGroupName:          "logfile.log",
		TimestampRegex:        "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}+)",
		TimestampLayout:       []string{"02 Jan 2006 15:04:05"},
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_regex}",
	}

	err = fileConfig.init()
	assert.NotNil(t, err)
	assert.Equal(t, "", fileConfig.LogGroupClass)
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
	timestampLayout := []string{"02 Jan 2006 15:04:05"}
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
	timestampLayout := []string{"1 2 15:04:05"}
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

func TestTimestampParserDefault(t *testing.T) {
	// Check when timestamp_format is "%b %d %H:%M:%S"
	// %d and %-d are both treated as s{0,1}\\d{1,2}
	timestampRegex := "(\\w{3} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})"
	timestampLayout := []string{"test", "Jan 2 15:04:05"}
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

	// make sure layout is compatible for "Sep 9", "Sep  9" , "Sep 09", "Sep  09" options
	logEntry := fmt.Sprintf("Sep 9 02:00:43  ip-10-4-213-132 \n")
	timestamp := fileConfig.timestampFromLogLine(logEntry)
	assert.Equal(t, 02, timestamp.Hour())
	assert.Equal(t, 00, timestamp.Minute())

	logEntry = fmt.Sprintf("Sep  9 02:00:43  ip-10-4-213-132 \n")
	timestamp = fileConfig.timestampFromLogLine(logEntry)
	assert.Equal(t, 02, timestamp.Hour())
	assert.Equal(t, 00, timestamp.Minute())

	logEntry = fmt.Sprintf("Sep 09 02:00:43  ip-10-4-213-132 \n")
	timestamp = fileConfig.timestampFromLogLine(logEntry)
	assert.Equal(t, 02, timestamp.Hour())
	assert.Equal(t, 00, timestamp.Minute())

	logEntry = fmt.Sprintf("Sep  09 02:00:43  ip-10-4-213-132 \n")
	timestamp = fileConfig.timestampFromLogLine(logEntry)
	assert.Equal(t, 02, timestamp.Hour())
	assert.Equal(t, 00, timestamp.Minute())

}

func TestTimestampParserWithFracSeconds(t *testing.T) {
	timestampRegex := "(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2},(\\d{1,9}) \\w{3})"
	timestampLayout := []string{"02 Jan 2006 15:04:05,.000 MST"}
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

func TestNonAllowlistedTimezone(t *testing.T) {
	fileConfig := &FileConfig{
		Timezone: "EST",
	}

	err := fileConfig.init()
	assert.NoError(t, err)

	assert.Equal(t, time.Local, fileConfig.TimezoneLoc, "The timezone location should be in local timezone.")
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
		Type:       includeFilterType,
		Expression: "StatusCode: [4-5]\\d\\d",
	}
	filter2 := LogFilter{
		Type:       excludeFilterType,
		Expression: "Some expression that (will|won't) compile",
	}

	fileConfig := &FileConfig{
		FilePath: "/tmp/logfile.log",
		Filters:  []*LogFilter{&filter1, &filter2},
	}

	err := fileConfig.init()
	assert.NoError(t, err)

	assert.Len(t, fileConfig.Filters, 2)
	f := fileConfig.Filters[0]
	assert.NotNil(t, f.expressionP)
	assert.Equal(t, filter1.Type, f.Type)
	assert.Equal(t, filter1.Expression, f.Expression)
	f = fileConfig.Filters[1]
	assert.NotNil(t, f.expressionP)
	assert.Equal(t, filter2.Type, f.Type)
	assert.Equal(t, filter2.Expression, f.Expression)
}

func TestFileConfigInitWithFiltersFails(t *testing.T) {
	fileConfig := &FileConfig{
		FilePath: "/tmp/logfile.log",
		Filters: []*LogFilter{
			{
				Type:       excludeFilterType,
				Expression: "Some expression that (will|won't) compile",
			},
			{
				Type:       includeFilterType,
				Expression: "StatusCode: ([4-5]\\d\\d", // invalid regexp
			},
		},
	}

	err := fileConfig.init()
	assert.Error(t, err)
	assert.Equal(t, "filter regex has issue, regexp: Compile( StatusCode: ([4-5]\\d\\d ): error parsing regexp: missing closing ): `StatusCode: ([4-5]\\d\\d`", err.Error())
}

func TestLogEmptyFilters(t *testing.T) {
	assertPublishedForFilters(t, []*LogFilter{}, "foo")
	assertPublishedForFilters(t, []*LogFilter{}, "Some other log message")
}

func TestLogNilFilters(t *testing.T) {
	assertPublishedForFilters(t, nil, "foo")
	assertPublishedForFilters(t, nil, "Some other log message")
}

func TestLogIncludeFilter(t *testing.T) {
	filters := initializeLogFilters(t, []*LogFilter{{
		Type:       includeFilterType,
		Expression: "StatusCode: [4-5]\\d\\d",
	}})

	assertPublishedForFilters(t, filters, "API responded with [StatusCode: 500] for call to /foo/bar")
	assertNotPublishedForFilters(t, filters, "This is another log message that doesn't match")
}

func TestLogExcludeFilter(t *testing.T) {
	filters := initializeLogFilters(t, []*LogFilter{{
		Type:       excludeFilterType,
		Expression: "StatusCode: [4-5]\\d\\d",
	}})
	assertNotPublishedForFilters(t, filters, "API responded with [StatusCode: 500] for call to /foo/bar")
	assertPublishedForFilters(t, filters, "This is another log message that doesn't match")
}

func TestLogIncludeThenExcludeFilter(t *testing.T) {
	filters := initializeLogFilters(t, []*LogFilter{
		{
			Type:       includeFilterType,
			Expression: "search_(\\w+)",
		},
		{
			Type:       excludeFilterType,
			Expression: "StatusCode: [4-5]\\d\\d",
		},
	})
	assertNotPublishedForFilters(t, filters, "API responded with [StatusCode: 500] for call to /foo/bar")
	assertPublishedForFilters(t, filters, "Submitted request to application search_FooBarBaz1")
	// If the log message matches both, it should match the inclusion filter, then proceed to the exclusion filter
	assertNotPublishedForFilters(t, filters, "Here is a log for search_Abc123 that also has a status code of (StatusCode: 425) and that's it.")
	// If the log message matches neither, because this config has an inclusion filter, drop the log
	assertNotPublishedForFilters(t, filters, "Some other log that doesn't match either expression")
}

func TestLogExcludeThenIncludeFilter(t *testing.T) {
	filters := initializeLogFilters(t, []*LogFilter{
		{
			Type:       excludeFilterType,
			Expression: "search_(\\w+)",
		},
		{
			Type:       includeFilterType,
			Expression: "StatusCode: [4-5]\\d\\d",
		},
	})
	assertPublishedForFilters(t, filters, "API responded with [StatusCode: 500] for call to /foo/bar")
	assertNotPublishedForFilters(t, filters, "Submitted request to application search_FooBarBaz1")
	// If the log message matches both, it should match the exclusion filter, which indicates that we should drop the log
	assertNotPublishedForFilters(t, filters, "Here is a log for search_Abc123 that also has a status code of (StatusCode: 425) and that's it.")
	// If the log message matches neither, because this config has an inclusion filter, drop the log
	assertNotPublishedForFilters(t, filters, "Some other log that doesn't match either expression")
}

func TestLogFilterMultipleExclusionExpressions(t *testing.T) {
	filters := initializeLogFilters(t, []*LogFilter{
		{
			Type:       excludeFilterType,
			Expression: "search_(\\w+)",
		},
		{
			Type:       excludeFilterType,
			Expression: "StatusCode: [4-5]\\d\\d",
		},
	})
	assertPublishedForFilters(t, filters, "Some other log that doesn't match either expression")
	assertNotPublishedForFilters(t, filters, "API responded with [StatusCode: 500] for call to /foo/bar")
	assertNotPublishedForFilters(t, filters, "Submitted request to application search_FooBarBaz1")
	assertNotPublishedForFilters(t, filters, "Here is a log for search_Abc123 that also has a status code of (StatusCode: 425) and that's it.")
}

func TestLogFilterMultipleInclusionExpressions(t *testing.T) {
	filters := initializeLogFilters(t, []*LogFilter{
		{
			Type:       includeFilterType,
			Expression: "search_(\\w+)",
		},
		{
			Type:       includeFilterType,
			Expression: "StatusCode: [4-5]\\d\\d",
		},
	})
	assertNotPublishedForFilters(t, filters, "Some other log that doesn't match either expression")
	assertNotPublishedForFilters(t, filters, "API responded with [StatusCode: 500] for call to /foo/bar")
	assertNotPublishedForFilters(t, filters, "Submitted request to application search_FooBarBaz1")
	assertPublishedForFilters(t, filters, "Here is a log for search_Abc123 that also has a status code of (StatusCode: 425) and that's it.")
}

func BenchmarkLogFilterSimpleInclude(b *testing.B) {
	filters := initializeLogFiltersForBenchmarks(b, []*LogFilter{
		{
			Type:       includeFilterType,
			Expression: "StatusCode: [4-5]\\d\\d",
		},
	})
	event := LogEvent{
		msg: "API responded with [StatusCode: 409] for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		ShouldPublish("foo", "bar", filters, event)
	}
}

func BenchmarkLogFilterSimpleIncludeNotMatch(b *testing.B) {
	filters := initializeLogFiltersForBenchmarks(b, []*LogFilter{
		{
			Type:       includeFilterType,
			Expression: "StatusCode: [4-5]\\d\\d",
		},
	})
	event := LogEvent{
		msg: "API responded with [StatusCode: 209] for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		ShouldPublish("foo", "bar", filters, event)
	}
}

func BenchmarkLogFilterSimpleExclude(b *testing.B) {
	filters := initializeLogFiltersForBenchmarks(b, []*LogFilter{
		{
			Type:       excludeFilterType,
			Expression: "StatusCode: [4-5]\\d\\d",
		},
	})
	event := LogEvent{
		msg: "API responded with [StatusCode: 409] for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		ShouldPublish("foo", "bar", filters, event)
	}
}

func BenchmarkLogFilterSimpleExcludeNotMatch(b *testing.B) {
	filters := initializeLogFiltersForBenchmarks(b, []*LogFilter{
		{
			Type:       excludeFilterType,
			Expression: "StatusCode: [4-5]\\d\\d",
		},
	})
	event := LogEvent{
		msg: "API responded with [StatusCode: 209] for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		ShouldPublish("foo", "bar", filters, event)
	}
}

func BenchmarkLogFilterIncludeThenMatchExclude(b *testing.B) {
	filters := initializeLogFiltersForBenchmarks(b, []*LogFilter{
		{
			Type:       includeFilterType,
			Expression: "search_(\\w+)",
		},
		{
			Type:       excludeFilterType,
			Expression: "StatusCode: [4-5]\\d\\d",
		},
	})
	event := LogEvent{
		msg: "API responded with [StatusCode: 409] for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		ShouldPublish("foo", "bar", filters, event)
	}
}

func BenchmarkLogFilterExclusionsDoNotDropUnmatchedLog(b *testing.B) {
	filters := initializeLogFiltersForBenchmarks(b, []*LogFilter{
		{
			Type:       excludeFilterType,
			Expression: "search_(\\w+)",
		},
		{
			Type:       excludeFilterType,
			Expression: "StatusCode: [4-5]\\d\\d",
		},
	})
	event := LogEvent{
		msg: "Some other log that doesn't match either expression",
	}
	for i := 0; i < b.N; i++ {
		ShouldPublish("foo", "bar", filters, event)
	}
}

func BenchmarkLogFilterMatchesMultipleInclusionExpressions(b *testing.B) {
	filters := initializeLogFiltersForBenchmarks(b, []*LogFilter{
		{
			Type:       includeFilterType,
			Expression: "(WARN|ERROR)",
		},
		{
			Type:       includeFilterType,
			Expression: "StatusCode: [4-5]\\d{2} for call to (/(\\w)+)+",
		},
	})
	event := LogEvent{
		msg: "2021-12-16 21:45:13 - WARN: API responded with StatusCode: 502 for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		ShouldPublish("foo", "bar", filters, event)
	}
}

func BenchmarkLogFilterDoesNotMatchMultipleInclusionExpressions(b *testing.B) {
	filters := initializeLogFiltersForBenchmarks(b, []*LogFilter{
		{
			Type:       includeFilterType,
			Expression: "(WARN|ERROR)",
		},
		{
			Type:       includeFilterType,
			Expression: "StatusCode: [4-5]\\d{2} for call to (/(\\w)+)+",
		},
	})
	event := LogEvent{
		msg: "2021-12-16 21:45:13 - DEBUG: API responded with StatusCode: 200 for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		ShouldPublish("foo", "bar", filters, event)
	}
}

func BenchmarkLogFilterMatchesComplexExpression(b *testing.B) {
	filters := initializeLogFiltersForBenchmarks(b, []*LogFilter{
		{
			Type:       includeFilterType,
			Expression: "((WARN|ERROR)|(StatusCode: [4-5]\\d{2} for call to (/(\\w)+)+))",
		},
	})
	event := LogEvent{
		msg: "2021-12-16 21:45:13 - DEBUG: API responded with StatusCode: 502 for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		ShouldPublish("foo", "bar", filters, event)
	}
}

func BenchmarkLogFilterDoesNotMatchComplexExpression(b *testing.B) {
	filters := initializeLogFiltersForBenchmarks(b, []*LogFilter{
		{
			Type:       includeFilterType,
			Expression: "((WARN|ERROR)|(StatusCode: [4-5]\\d{2} for call to (/(\\w)+)+))",
		},
	})
	event := LogEvent{
		msg: "2021-12-16 21:45:13 - DEBUG: API responded with StatusCode: 209 for call to /foo/bar",
	}
	for i := 0; i < b.N; i++ {
		ShouldPublish("foo", "bar", filters, event)
	}
}

func assertPublishedForFilters(t *testing.T, filters []*LogFilter, msg string) {
	res := ShouldPublish("foo", "bar", filters, LogEvent{
		msg: msg,
	})
	assert.True(t, res)
}

func assertNotPublishedForFilters(t *testing.T, filters []*LogFilter, msg string) {
	res := ShouldPublish("foo", "bar", filters, LogEvent{
		msg: msg,
	})
	assert.False(t, res)
}

func initializeLogFilters(t *testing.T, filters []*LogFilter) []*LogFilter {
	for _, f := range filters {
		err := f.init()
		assert.NoError(t, err)
	}
	return filters
}

func initializeLogFiltersForBenchmarks(b *testing.B, filters []*LogFilter) []*LogFilter {
	defer b.ResetTimer()
	for _, f := range filters {
		err := f.init()
		assert.NoError(b, err)
	}
	return filters
}
