// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslogrouterprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func newTestLogs(attrs map[string]interface{}) plog.Logs {
	return newTestLogsWithBody(attrs, "")
}

func newTestLogsWithBody(attrs map[string]interface{}, body string) plog.Logs {
	ld := plog.NewLogs()
	rec := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	if body != "" {
		rec.Body().SetStr(body)
	}
	for k, v := range attrs {
		switch val := v.(type) {
		case string:
			rec.Attributes().PutStr(k, val)
		case int64:
			rec.Attributes().PutInt(k, val)
		}
	}
	return ld
}

func recordCount(ld plog.Logs) int {
	count := 0
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		for j := 0; j < ld.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			count += ld.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
		}
	}
	return count
}

func TestHostnameGlobMatch(t *testing.T) {
	p := newProcessor(&Config{
		Rule: MatchRule{Hostname: "web-*"},
	}, zap.NewNop())
	result, err := p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"hostname": "web-01"}))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))

	result, err = p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"hostname": "db-01"}))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))
}

func TestFacilityExactMatch(t *testing.T) {
	fac := 1
	p := newProcessor(&Config{
		Rule: MatchRule{Facility: &fac},
	}, zap.NewNop())
	result, err := p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"facility": int64(1)}))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))

	result, err = p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"facility": int64(2)}))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))
}

func TestAppNameGlobMatch(t *testing.T) {
	p := newProcessor(&Config{
		Rule: MatchRule{AppName: "nginx*"},
	}, zap.NewNop())
	result, err := p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"appname": "nginx-proxy"}))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))

	result, err = p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"appname": "apache"}))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))
}

func TestANDLogic(t *testing.T) {
	fac := 1
	p := newProcessor(&Config{
		Rule: MatchRule{Hostname: "web-*", Facility: &fac},
	}, zap.NewNop())
	result, err := p.processLogs(context.Background(), newTestLogs(map[string]interface{}{
		"hostname": "web-01",
		"facility": int64(1),
	}))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))

	// hostname matches but facility doesn't
	result, err = p.processLogs(context.Background(), newTestLogs(map[string]interface{}{
		"hostname": "web-01",
		"facility": int64(2),
	}))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))
}

func TestDefaultPassesUnmatchedOnly(t *testing.T) {
	fac := 1
	p := newProcessor(&Config{
		IsDefault: true,
		AllRules: []MatchRule{
			{Hostname: "web-*"},
			{Facility: &fac},
		},
	}, zap.NewNop())
	// matches first rule -> dropped
	result, err := p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"hostname": "web-01"}))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))

	// matches second rule -> dropped
	result, err = p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"facility": int64(1)}))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))

	// matches no rule -> passed
	result, err = p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"hostname": "db-01", "facility": int64(5)}))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))
}

func TestPriorRulesFirstMatchWins(t *testing.T) {
	p := newProcessor(&Config{
		Rule:       MatchRule{Hostname: "web-*"},
		PriorRules: []MatchRule{{Hostname: "web-01"}},
	}, zap.NewNop())
	// matches Rule but also matches PriorRule -> dropped (prior rule claimed it)
	result, err := p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"hostname": "web-01"}))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))

	// matches Rule, no PriorRule match -> passed
	result, err = p.processLogs(context.Background(), newTestLogs(map[string]interface{}{"hostname": "web-02"}))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))
}

func TestEmptyBatchReturnsEmpty(t *testing.T) {
	p := newProcessor(&Config{
		Rule: MatchRule{Hostname: "web-*"},
	}, zap.NewNop())
	result, err := p.processLogs(context.Background(), plog.NewLogs())
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))
}

func TestListenerExcludeFilter(t *testing.T) {
	p := newProcessor(&Config{
		IsDefault:       true,
		ListenerFilters: []Filter{{Type: "exclude", Expression: "healthcheck"}},
	}, zap.NewNop())
	result, err := p.processLogs(context.Background(), newTestLogsWithBody(nil, "healthcheck OK"))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))

	result, err = p.processLogs(context.Background(), newTestLogsWithBody(nil, "request processed"))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))
}

func TestListenerIncludeFilter(t *testing.T) {
	p := newProcessor(&Config{
		IsDefault:       true,
		ListenerFilters: []Filter{{Type: "include", Expression: "error|warn"}},
	}, zap.NewNop())
	result, err := p.processLogs(context.Background(), newTestLogsWithBody(nil, "error: disk full"))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))

	result, err = p.processLogs(context.Background(), newTestLogsWithBody(nil, "info: all good"))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))
}

func TestListenerExcludeBeforeInclude(t *testing.T) {
	p := newProcessor(&Config{
		IsDefault: true,
		ListenerFilters: []Filter{
			{Type: "exclude", Expression: "DEBUG"},
			{Type: "include", Expression: "DEBUG|ERROR"},
		},
	}, zap.NewNop())
	// matches exclude -> dropped even though it also matches include
	result, err := p.processLogs(context.Background(), newTestLogsWithBody(nil, "DEBUG something"))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))

	// matches include only -> passed
	result, err = p.processLogs(context.Background(), newTestLogsWithBody(nil, "ERROR something"))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))
}

func TestRuleExcludeFilter(t *testing.T) {
	p := newProcessor(&Config{
		Rule:        MatchRule{Hostname: "web-*"},
		RuleFilters: []Filter{{Type: "exclude", Expression: "DEBUG"}},
	}, zap.NewNop())
	result, err := p.processLogs(context.Background(), newTestLogsWithBody(
		map[string]interface{}{"hostname": "web-01"}, "DEBUG verbose output"))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))

	result, err = p.processLogs(context.Background(), newTestLogsWithBody(
		map[string]interface{}{"hostname": "web-01"}, "ERROR something broke"))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))
}

func TestRuleIncludeFilter(t *testing.T) {
	p := newProcessor(&Config{
		Rule:        MatchRule{Hostname: "web-*"},
		RuleFilters: []Filter{{Type: "include", Expression: "error|crit"}},
	}, zap.NewNop())
	result, err := p.processLogs(context.Background(), newTestLogsWithBody(
		map[string]interface{}{"hostname": "web-01"}, "error: timeout"))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))

	result, err = p.processLogs(context.Background(), newTestLogsWithBody(
		map[string]interface{}{"hostname": "web-01"}, "info: request ok"))
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount(result))
}

func TestDefaultPipelineLogsDiagnosticForUnparsedMessages(t *testing.T) {
	core, observed := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	p := newProcessor(&Config{
		IsDefault: true,
		AllRules:  []MatchRule{{Hostname: "web-*"}},
	}, logger)

	// Record without hostname attribute (simulates parse failure)
	result, err := p.processLogs(context.Background(), newTestLogsWithBody(nil, "<85>May 04 12:00:00 myhost app: test"))
	require.NoError(t, err)
	assert.Equal(t, 1, recordCount(result))
	assert.Equal(t, 1, observed.Len())
	assert.Contains(t, observed.All()[0].Message, "missing parsed attributes")
	assert.Equal(t, "<85>May 04 12:00:00 myhost app: test", observed.All()[0].ContextMap()["body"])
}
