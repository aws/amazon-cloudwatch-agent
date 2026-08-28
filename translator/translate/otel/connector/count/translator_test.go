// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package count

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslatorID(t *testing.T) {
	tr := NewTranslator(common.DbiConnectorDbload)
	assert.Equal(t, "count/dbi_dbload", tr.ID().String())
}

func TestTranslateDbload(t *testing.T) {
	tr := NewTranslator(common.DbiConnectorDbload)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	countCfg := cfg.(*countconnector.Config)
	assert.Len(t, countCfg.Logs, 8)
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.by_app")
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.by_db")
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.by_host")
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.by_sql")
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.by_sql_wait")
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.by_user")
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.by_wait")
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.count")
}

func TestTranslateUnsupported(t *testing.T) {
	tr := NewTranslator("unsupported")
	_, err := tr.Translate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported count connector config")
}
