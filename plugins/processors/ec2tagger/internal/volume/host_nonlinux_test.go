// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !linux

package volume

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostProvider(t *testing.T) {
	p := newHostProvider()
	got, err := p.DeviceToSerialMap()
	assert.Error(t, err)
	assert.Nil(t, got)
}
