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
	tr := NewTranslator(common.DbiConnectorDbload, 0)
	assert.Equal(t, "count/dbi_dbload_0", tr.ID().String())

	tr = NewTranslator(common.DbiConnectorDbload, 3)
	assert.Equal(t, "count/dbi_dbload_3", tr.ID().String())
}

func TestTranslateDbload(t *testing.T) {
	tr := NewTranslator(common.DbiConnectorDbload, 0)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	countCfg, ok := cfg.(*countconnector.Config)
	require.True(t, ok)
	require.Len(t, countCfg.Logs, 8)
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.by_wait")
	assert.Contains(t, countCfg.Logs, "postgresql.active_sessions.count")
}

func TestTranslateUnsupported(t *testing.T) {
	tr := NewTranslator("unsupported", 0)
	_, err := tr.Translate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported count connector config")
}
