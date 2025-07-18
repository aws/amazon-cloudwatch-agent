// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package wineventlog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventLogFilterInit(t *testing.T) {
	exp := "(foo|bar|baz)"
	filter, err := initEventLogFilter(includeFilterType, exp)
	assert.NoError(t, err)
	assert.NotNil(t, filter.expressionP)
	filter, err = initEventLogFilter(excludeFilterType, exp)
	assert.NoError(t, err)
	assert.NotNil(t, filter.expressionP)
}

func TestLogEventFilterInitInvalidType(t *testing.T) {
	_, err := initEventLogFilter("something wrong", "(foo|bar|baz)")
	assert.Error(t, err)
}

func TestLogEventFilterInitInvalidRegex(t *testing.T) {
	_, err := initEventLogFilter(excludeFilterType, "abc)")
	assert.Error(t, err)
}

func TestLogEventFilterShouldPublishInclude(t *testing.T) {
	exp := "(foo|bar|baz)"
	filter, err := initEventLogFilter(includeFilterType, exp)
	assert.NoError(t, err)

	assertShouldPublish(t, filter, "foo bar baz")
	assertShouldNotPublish(t, filter, "something else")
}

func TestEventLogFilterShouldPublishExclude(t *testing.T) {
	exp := "(foo|bar|baz)"
	filter, err := initEventLogFilter(excludeFilterType, exp)
	assert.NoError(t, err)

	assertShouldNotPublish(t, filter, "foo bar baz")
	assertShouldPublish(t, filter, "something else")
}

func BenchmarkEventLogFilterShouldPublish(b *testing.B) {
	exp := "(foo|bar|baz)"
	filter, err := initEventLogFilter(excludeFilterType, exp)
	assert.NoError(b, err)
	b.ResetTimer()

	msg := "foo bar baz"

	for i := 0; i < b.N; i++ {
		filter.ShouldPublish(msg)
	}
}

func BenchmarkEventLogFilterShouldNotPublish(b *testing.B) {
	exp := "(foo|bar|baz)"
	filter, err := initEventLogFilter(excludeFilterType, exp)
	assert.NoError(b, err)
	b.ResetTimer()

	msg := "something else"

	for i := 0; i < b.N; i++ {
		filter.ShouldPublish(msg)
	}
}

func initEventLogFilter(filterType, expressionStr string) (EventFilter, error) {
	filter := EventFilter{
		Type:       filterType,
		Expression: expressionStr,
	}
	err := filter.init()
	return filter, err
}

func assertShouldPublish(t *testing.T, filter EventFilter, msg string) {
	res := filter.ShouldPublish(msg)
	assert.True(t, res)
}

func assertShouldNotPublish(t *testing.T, filter EventFilter, msg string) {
	res := filter.ShouldPublish(msg)
	assert.False(t, res)
}
