// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

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
