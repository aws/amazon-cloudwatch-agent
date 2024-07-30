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
