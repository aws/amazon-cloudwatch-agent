package windows

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"

	"github.com/stretchr/testify/assert"
)

func TestProcessor_ToMap(t *testing.T) {
	expectedKey := "Processor"
	expectedValue := map[string]interface{}{"resources": []string{"_Total"}, "measurement": []string{"% Processor Time", "% User Time", "% Idle Time", "% Interrupt Time"}}
	ctx := &runtime.Context{}
	conf := new(Processor)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
