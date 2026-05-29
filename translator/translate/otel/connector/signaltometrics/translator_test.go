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
	tr := NewTranslator(common.DbiConnectorTopsql, 0)
	assert.Equal(t, "signaltometrics/dbi_topsql_0", tr.ID().String())

	tr = NewTranslator(common.DbiConnectorTopsql, 2)
	assert.Equal(t, "signaltometrics/dbi_topsql_2", tr.ID().String())
}

func TestTranslateTopsql(t *testing.T) {
	tr := NewTranslator(common.DbiConnectorTopsql, 0)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	stmCfg, ok := cfg.(*signaltometricsconfig.Config)
	require.True(t, ok)
	require.Len(t, stmCfg.Logs, 8)
	assert.Equal(t, "postgresql.calls", stmCfg.Logs[0].Name)
	assert.Equal(t, "postgresql.local_blks_read", stmCfg.Logs[7].Name)
}

func TestTranslateUnsupported(t *testing.T) {
	tr := NewTranslator("unsupported", 0)
	_, err := tr.Translate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported signaltometrics connector config")
}
