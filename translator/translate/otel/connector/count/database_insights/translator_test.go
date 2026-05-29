// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package database_insights

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslatorID(t *testing.T) {
	tr := NewTranslator("postgresql", 0)
	assert.Equal(t, "count/dbload_0", tr.ID().String())

	tr = NewTranslator("postgresql", 3)
	assert.Equal(t, "count/dbload_3", tr.ID().String())
}

func TestTranslatePostgresql(t *testing.T) {
	tr := NewTranslator("postgresql", 0)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	countCfg, ok := cfg.(*countconnector.Config)
	require.True(t, ok)
	require.Len(t, countCfg.Logs, 8)
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.by_wait")
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.count")
}

func TestTranslateUnsupportedEngine(t *testing.T) {
	tr := NewTranslator("unsupported", 0)
	_, err := tr.Translate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to read count connector config for engine unsupported")
}
