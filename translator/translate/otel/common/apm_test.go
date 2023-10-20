// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAPMKubernetes(t *testing.T) {
	assert.False(t, IsAPMKubernetes())
	t.Setenv(KubernetesEnvVar, "TEST")
	assert.True(t, IsAPMKubernetes())
}
