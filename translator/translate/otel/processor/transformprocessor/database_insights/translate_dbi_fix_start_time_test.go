package database_insights

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixStartTimeTranslator_ID(t *testing.T) {
	tr := NewFixStartTimeTranslator()
	assert.Equal(t, "transform/dbi_fix_start_time", tr.ID().String())
}

func TestFixStartTimeTranslator_Translate(t *testing.T) {
	tr := NewFixStartTimeTranslator()
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	tfCfg := cfg.(*transformprocessor.Config)

	require.Len(t, tfCfg.MetricStatements, 1)
	assert.Equal(t, "datapoint", string(tfCfg.MetricStatements[0].Context))
	require.Len(t, tfCfg.MetricStatements[0].Statements, 7)
	assert.Equal(t, "set(datapoint.start_time_unix_nano, datapoint.time_unix_nano) where datapoint.start_time_unix_nano == 0", tfCfg.MetricStatements[0].Statements[0])
	assert.Equal(t, `replace_match(datapoint.attributes["postgresql.wait_event_type"], "", "CPU")`, tfCfg.MetricStatements[0].Statements[1])
	assert.Equal(t, `replace_match(datapoint.attributes["postgresql.wait_event"], "", "CPU")`, tfCfg.MetricStatements[0].Statements[2])
	assert.Equal(t, `replace_match(datapoint.attributes["postgresql.application_name"], "", "unknown")`, tfCfg.MetricStatements[0].Statements[3])
	assert.Equal(t, `replace_match(datapoint.attributes["network.peer.address"], "", "unknown")`, tfCfg.MetricStatements[0].Statements[4])
	assert.Equal(t, `replace_match(datapoint.attributes["postgresql.query_id"], "", "unknown")`, tfCfg.MetricStatements[0].Statements[5])
	assert.Equal(t, `replace_match(datapoint.attributes["user.name"], "", "unknown")`, tfCfg.MetricStatements[0].Statements[6])
}
