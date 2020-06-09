package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsUnknownKey(t *testing.T) {
	for _, knownConfigKey := range knownConfigKeys {
		assert.Equal(t, false, isUnknownKey(knownConfigKey))
	}
	assert.Equal(t, true, isUnknownKey("RandomUnknownKey"))
}
