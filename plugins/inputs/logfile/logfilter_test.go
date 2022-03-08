// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogFilterInit(t *testing.T) {
	exp := "(foo|bar|baz)"
	filter, err := initLogFilter(includeFilterType, exp)
	assert.NoError(t, err)
	assert.NotNil(t, filter.expressionP)
	filter, err = initLogFilter(excludeFilterType, exp)
	assert.NoError(t, err)
	assert.NotNil(t, filter.expressionP)
}

func TestLogFilterInitInvalidType(t *testing.T) {
	_, err := initLogFilter("something wrong", "(foo|bar|baz)")
	assert.Error(t, err)
}

func TestLogFilterInitInvalidRegex(t *testing.T) {
	_, err := initLogFilter(excludeFilterType, "abc)")
	assert.Error(t, err)
}

func TestLogFilterShouldPublishInclude(t *testing.T) {
	exp := "(foo|bar|baz)"
	filter, err := initLogFilter(includeFilterType, exp)
	assert.NoError(t, err)

	assertShouldPublish(t, filter, "foo bar baz")
	assertShouldNotPublish(t, filter, "something else")
}

func TestLogFilterShouldPublishExclude(t *testing.T) {
	exp := "(foo|bar|baz)"
	filter, err := initLogFilter(excludeFilterType, exp)
	assert.NoError(t, err)

	assertShouldNotPublish(t, filter, "foo bar baz")
	assertShouldPublish(t, filter, "something else")
}

func BenchmarkLogFilterShouldPublish(b *testing.B) {
	exp := "(foo|bar|baz)"
	filter, err := initLogFilter(excludeFilterType, exp)
	assert.NoError(b, err)
	b.ResetTimer()

	event := LogEvent{
		msg: "foo bar baz",
	}

	for i := 0; i < b.N; i++ {
		filter.ShouldPublish(event)
	}
}

func BenchmarkLogFilterShouldNotPublish(b *testing.B) {
	exp := "(foo|bar|baz)"
	filter, err := initLogFilter(excludeFilterType, exp)
	assert.NoError(b, err)
	b.ResetTimer()

	event := LogEvent{
		msg: "something else",
	}

	for i := 0; i < b.N; i++ {
		filter.ShouldPublish(event)
	}
}

func initLogFilter(filterType, expressionStr string) (LogFilter, error) {
	filter := LogFilter{
		Type:       filterType,
		Expression: expressionStr,
	}
	err := filter.init()
	return filter, err
}

func assertShouldPublish(t *testing.T, filter LogFilter, msg string) {
	res := filter.ShouldPublish(LogEvent{
		msg: msg,
	})
	assert.True(t, res)
}

func assertShouldNotPublish(t *testing.T, filter LogFilter, msg string) {
	res := filter.ShouldPublish(LogEvent{
		msg: msg,
	})
	assert.False(t, res)
}
