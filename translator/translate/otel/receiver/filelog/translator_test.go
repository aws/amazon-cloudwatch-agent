package filelog

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestTranslator_ID(t *testing.T) {
	tr := NewTranslator(WithIndex(0))
	assert.Equal(t, component.MustNewIDWithName("filelog", "postgresql_0"), tr.ID())
}

func TestTranslator_Translate(t *testing.T) {
	tr := NewTranslator(
		WithFilePath("/var/log/postgresql/postgresql.log"),
		WithIndex(0),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	flCfg := cfg.(*filelogreceiver.FileLogConfig)
	assert.Equal(t, []string{"/var/log/postgresql/postgresql.log"}, flCfg.InputConfig.Include)
	assert.Equal(t, "end", flCfg.InputConfig.StartAt)
	assert.Equal(t, "utf-8", flCfg.InputConfig.Encoding)
}
