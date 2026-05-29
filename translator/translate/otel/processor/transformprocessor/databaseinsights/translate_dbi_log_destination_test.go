// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package databaseinsights

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogsRoutingTranslator_ID(t *testing.T) {
	tr := NewLogsRoutingTranslator("my-db", "raw-events", 0)
	assert.Equal(t, "transform/dbi_logs_raw-events_0", tr.ID().String())
}

func TestLogsRoutingTranslator_Translate(t *testing.T) {
	tr := NewLogsRoutingTranslator("my-db", "raw-events", 0)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	tfCfg := cfg.(*transformprocessor.Config)

	require.Len(t, tfCfg.LogStatements, 1)
	assert.Equal(t, "resource", string(tfCfg.LogStatements[0].Context))
	require.Len(t, tfCfg.LogStatements[0].Statements, 2)
	assert.Equal(t, `set(resource.attributes["aws.log.group.name"], "/aws/self-managed-database-insights/postgresql/raw-events")`, tfCfg.LogStatements[0].Statements[0])
	assert.Equal(t, `set(resource.attributes["aws.log.stream.name"], Concat([resource.attributes["host.id"], "my-db"], "/"))`, tfCfg.LogStatements[0].Statements[1])
}
