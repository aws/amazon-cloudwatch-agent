// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAppSignalsKubernetes(t *testing.T) {
	assert.False(t, IsAppSignalsKubernetes())
	t.Setenv(KubernetesEnvVar, "TEST")
	assert.True(t, IsAppSignalsKubernetes())
}
