package linux

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"

	"github.com/stretchr/testify/assert"
)

func TestNetStat_ToMap(t *testing.T) {
	expectedKey := "netstat"
	expectedValue := map[string]interface{}{"measurement": []string{"tcp_established", "tcp_time_wait"}}
	ctx := &runtime.Context{}
	conf := new(NetStat)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
