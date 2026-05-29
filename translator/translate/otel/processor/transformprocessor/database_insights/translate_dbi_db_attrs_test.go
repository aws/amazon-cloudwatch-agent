package database_insights

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceTranslator_ID(t *testing.T) {
	tr := NewResourceTranslator("my-db", 0)
	assert.Equal(t, "transform/dbi_resource_0", tr.ID().String())

	tr = NewResourceTranslator("my-db", 2)
	assert.Equal(t, "transform/dbi_resource_2", tr.ID().String())
}

func TestResourceTranslator_Translate(t *testing.T) {
	tr := NewResourceTranslator("my-db", 0)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	tfCfg := cfg.(*transformprocessor.Config)

	require.Len(t, tfCfg.MetricStatements, 1)
	assert.Equal(t, "resource", string(tfCfg.MetricStatements[0].Context))
	require.Len(t, tfCfg.MetricStatements[0].Statements, 2)
	assert.Equal(t, `set(resource.attributes["db.system.name"], "postgresql")`, tfCfg.MetricStatements[0].Statements[0])
	assert.Equal(t, `set(resource.attributes["db.instance.name"], "my-db")`, tfCfg.MetricStatements[0].Statements[1])

	require.Len(t, tfCfg.LogStatements, 1)
	assert.Equal(t, tfCfg.MetricStatements[0].Statements, tfCfg.LogStatements[0].Statements)
}
