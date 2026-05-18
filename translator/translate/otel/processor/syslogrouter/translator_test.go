// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslogrouter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/processor/syslogrouterprocessor"
)

func intPtr(v int) *int { return &v }

func TestTranslatorID(t *testing.T) {
	cfg := &syslogrouterprocessor.Config{Rule: syslogrouterprocessor.MatchRule{Hostname: "web-*"}}
	tt := NewTranslator("syslog_0_rule_0", cfg)
	assert.Equal(t, "awssyslogrouter/syslog_0_rule_0", tt.ID().String())
}

func TestTranslateReturnsConfig(t *testing.T) {
	fac := 4
	cfg := &syslogrouterprocessor.Config{
		Rule:       syslogrouterprocessor.MatchRule{Hostname: "web-*", Facility: &fac},
		PriorRules: []syslogrouterprocessor.MatchRule{{AppName: "nginx"}},
	}
	tt := NewTranslator("syslog_0_rule_1", cfg)
	got, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	assert.Equal(t, cfg, got)
}

func TestTranslateDefaultConfig(t *testing.T) {
	cfg := &syslogrouterprocessor.Config{
		IsDefault: true,
		AllRules:  []syslogrouterprocessor.MatchRule{{Hostname: "web-*"}, {Facility: intPtr(4)}},
	}
	tt := NewTranslator("syslog_0_default", cfg)
	got, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	gotCfg := got.(*syslogrouterprocessor.Config)
	assert.True(t, gotCfg.IsDefault)
	assert.Len(t, gotCfg.AllRules, 2)
}
