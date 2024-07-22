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

func TestK8sResolver(t *testing.T) {
	resolver := NewK8sResolver("test")
	assert.Equal(t, "k8s", resolver.Platform)
}

func TestEC2Resolver(t *testing.T) {
	resolver := NewEC2Resolver("test")
	assert.Equal(t, "ec2", resolver.Platform)
}

func TestNewGenericResolver(t *testing.T) {
	resolver := NewGenericResolver("")
	assert.Equal(t, "generic", resolver.Platform)
}
