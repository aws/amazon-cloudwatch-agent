// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package database_insights

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExcludeMonitorTranslator_ID(t *testing.T) {
	tr := NewExcludeMonitorTranslator("cw_monitor", 0)
	assert.Equal(t, "filter/dbi_exclude_monitor_0", tr.ID().String())
}

func TestExcludeMonitorTranslator_Translate(t *testing.T) {
	tr := NewExcludeMonitorTranslator("cw_monitor", 0)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	fCfg := cfg.(*filterprocessor.Config)
	require.NotNil(t, fCfg.Logs)
	require.Len(t, fCfg.Logs.LogConditions, 1)
	assert.Equal(t, `attributes["user.name"] == "cw_monitor" or attributes["postgresql.rolname"] == "cw_monitor"`, fCfg.Logs.LogConditions[0])
}
