// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEKSResolver(t *testing.T) {
	resolver := NewEKSResolver("test")
	assert.Equal(t, "eks", resolver.Platform)
}

func TestNewGenericResolver(t *testing.T) {
	resolver := NewGenericResolver("")
	assert.Equal(t, "generic", resolver.Platform)
}
