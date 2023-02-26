// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ServiceDiscoveryError(t *testing.T) {
	innerError := newServiceDiscoveryError("innerError", nil)
	assert.Equal(t, "innerError", innerError.Error())

	outerError := newServiceDiscoveryError("OuterError", &innerError)
	assert.Equal(t, "OuterError; original error: innerError", outerError.Error())
}
