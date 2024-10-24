// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithName(t *testing.T) {
	p := &NameProvider{name: "a"}
	opt := WithName("b")
	opt(p)
	assert.Equal(t, "b", p.Name())
}

func TestWithIndex(t *testing.T) {
	p := &IndexProvider{index: -1}
	opt := WithIndex(1)
	opt(p)
	assert.Equal(t, 1, p.Index())
}

func TestWithDestination(t *testing.T) {
	p := &DestinationProvider{destination: "a"}
	opt := WithDestination("b")
	opt(p)
	assert.Equal(t, "b", p.Destination())
}
