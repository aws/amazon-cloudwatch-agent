// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBackoffDuration(t *testing.T) {
	t.Parallel()

	duration := getBackoffDuration(-1)
	assert.Equal(t, defaultBackoffDuration, duration)

	for i := range sleeps {
		duration := getBackoffDuration(i)
		assert.Equal(t, sleeps[i], duration)
	}

	duration = getBackoffDuration(len(sleeps))
	assert.Equal(t, defaultBackoffDuration, duration)
}
