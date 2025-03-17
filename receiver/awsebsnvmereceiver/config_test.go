package awsebsnvmereceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	c := Config{}
	err := c.Validate()
	require.NotNil(t, err)
}
