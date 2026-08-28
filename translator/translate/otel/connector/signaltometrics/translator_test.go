// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltometrics

import (
	"testing"

	signaltometricsconfig "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslatorID(t *testing.T) {
	tr := NewTranslator(common.DbiConnectorTopsql)
	assert.Equal(t, "signaltometrics/dbi_topsql", tr.ID().String())
}

func TestTranslateTopsql(t *testing.T) {
	tr := NewTranslator(common.DbiConnectorTopsql)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	stmCfg := cfg.(*signaltometricsconfig.Config)
	assert.Len(t, stmCfg.Logs, 8)
	assert.Equal(t, "postgresql.calls", stmCfg.Logs[0].Name)
	assert.Equal(t, "postgresql.total_exec_time", stmCfg.Logs[1].Name)
	assert.Equal(t, "postgresql.total_plan_time", stmCfg.Logs[2].Name)
	assert.Equal(t, "postgresql.rows", stmCfg.Logs[3].Name)
	assert.Equal(t, "postgresql.shared_blks_hit", stmCfg.Logs[4].Name)
	assert.Equal(t, "postgresql.shared_blks_read", stmCfg.Logs[5].Name)
	assert.Equal(t, "postgresql.local_blks_hit", stmCfg.Logs[6].Name)
	assert.Equal(t, "postgresql.local_blks_read", stmCfg.Logs[7].Name)
}

func TestTranslateUnsupported(t *testing.T) {
	tr := NewTranslator("unsupported")
	_, err := tr.Translate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported signaltometrics connector config")
}
