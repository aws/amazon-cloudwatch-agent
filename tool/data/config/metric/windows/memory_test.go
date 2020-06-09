package windows

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"

	"github.com/stretchr/testify/assert"
)

func TestMemory_ToMap(t *testing.T) {
	expectedKey := "Memory"
	expectedValue := map[string]interface{}{"measurement": []string{"% Committed Bytes In Use"}}
	ctx := &runtime.Context{}
	conf := new(Memory)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
