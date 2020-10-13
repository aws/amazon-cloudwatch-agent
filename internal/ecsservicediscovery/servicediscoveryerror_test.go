package ecsservicediscovery

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_ServiceDiscoveryError(t *testing.T) {
	innerError := newServiceDiscoveryError("innerError", nil)
	assert.Equal(t, "innerError", innerError.Error())

	outerError := newServiceDiscoveryError("OuterError", &innerError)
	assert.Equal(t, "OuterError; original error: innerError", outerError.Error())
}
