// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tagutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEKSClusterName(t *testing.T) {
	// This will return empty string in test environment since there's no real EC2 metadata
	result := GetEKSClusterName()
	assert.Equal(t, "", result)
}
