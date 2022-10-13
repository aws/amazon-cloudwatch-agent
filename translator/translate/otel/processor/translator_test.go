package processor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
)

func TestTranslator(t *testing.T) {
	factory := componenttest.NewNopProcessorFactory()
	got := NewDefaultTranslator(factory)
	require.Equal(t, config.Type("nop"), got.Type())
	cfg, err := got.Translate(nil)
	require.NoError(t, err)
	require.Equal(t, factory.CreateDefaultConfig(), cfg)
}
