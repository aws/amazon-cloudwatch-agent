// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package atomiccounter

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestAtomicCounter_IncrementDecrementSimple(t *testing.T) {
	assert := assert.New(t)
	ac := NewAtomicCounter()
	beforeCount := ac.Get()

	ac.Increment()
	assert.Equal(beforeCount+1, ac.Get())

	ac.Increment()
	assert.Equal(beforeCount+2, ac.Get())

	ac.Decrement()
	assert.Equal(beforeCount+1, ac.Get())

	ac.Decrement()
	assert.Equal(beforeCount, ac.Get())
}