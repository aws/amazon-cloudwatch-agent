package windows

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"

	"github.com/stretchr/testify/assert"
)

func TestTCPv6_ToMap(t *testing.T) {
	expectedKey := "TCPv6"
	expectedValue := map[string]interface{}{"measurement": []string{"Connections Established"}}
	ctx := &runtime.Context{}
	conf := new(TCPv6)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
