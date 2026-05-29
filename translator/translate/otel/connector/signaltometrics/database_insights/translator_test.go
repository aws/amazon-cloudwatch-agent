// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package database_insights

import (
	"testing"

	signaltometricsconfig "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslatorID(t *testing.T) {
	tr := NewTranslator("postgresql", 0)
	assert.Equal(t, "signaltometrics/topsql_0", tr.ID().String())

	tr = NewTranslator("postgresql", 2)
	assert.Equal(t, "signaltometrics/topsql_2", tr.ID().String())
}

func TestTranslatePostgresql(t *testing.T) {
	tr := NewTranslator("postgresql", 0)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	stmCfg, ok := cfg.(*signaltometricsconfig.Config)
	require.True(t, ok)
	require.Len(t, stmCfg.Logs, 8)
	assert.Equal(t, "postgresql.calls", stmCfg.Logs[0].Name)
	assert.Equal(t, "postgresql.local_blks_read", stmCfg.Logs[7].Name)
}

func TestTranslateUnsupportedEngine(t *testing.T) {
	tr := NewTranslator("unsupported", 0)
	_, err := tr.Translate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to read signaltometrics connector config for engine unsupported")
}
