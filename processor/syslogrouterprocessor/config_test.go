// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslogrouterprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func intPtr(v int) *int { return &v }

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Rule: MatchRule{Hostname: "host1"},
	}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_DefaultNoRuleRequired(t *testing.T) {
	cfg := &Config{IsDefault: true}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_MissingMatchFields(t *testing.T) {
	cfg := &Config{
		Rule: MatchRule{},
	}
	assert.Error(t, cfg.Validate())
}

func TestValidate_FacilityOutOfRange(t *testing.T) {
	cfg := &Config{
		Rule: MatchRule{Facility: intPtr(24)},
	}
	assert.Error(t, cfg.Validate())

	cfg = &Config{
		Rule: MatchRule{Facility: intPtr(-1)},
	}
	assert.Error(t, cfg.Validate())
}

func TestValidate_FacilityValid(t *testing.T) {
	cfg := &Config{
		Rule: MatchRule{Facility: intPtr(0)},
	}
	assert.NoError(t, cfg.Validate())

	cfg = &Config{
		Rule: MatchRule{Facility: intPtr(23)},
	}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_InvalidFilterType(t *testing.T) {
	cfg := &Config{
		IsDefault:       true,
		ListenerFilters: []Filter{{Type: "drop", Expression: "test"}},
	}
	assert.Error(t, cfg.Validate())
}

func TestValidate_InvalidFilterRegex(t *testing.T) {
	cfg := &Config{
		IsDefault:       true,
		ListenerFilters: []Filter{{Type: "exclude", Expression: "[invalid"}},
	}
	assert.Error(t, cfg.Validate())
}

func TestValidate_ValidFilters(t *testing.T) {
	cfg := &Config{
		IsDefault:       true,
		ListenerFilters: []Filter{{Type: "exclude", Expression: "health[-_]?check"}},
		RuleFilters:     []Filter{{Type: "include", Expression: "error|warn"}},
	}
	assert.NoError(t, cfg.Validate())
}
