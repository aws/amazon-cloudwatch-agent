// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileRanges(t *testing.T) {
	r := FileRanges{}
	r.Add(FileRange{Start: 0, End: 10})
	r.Add(FileRange{Start: 20, End: 25})
	assert.Len(t, r.ranges, 2)
	r.Add(FileRange{Start: 10, End: 15})
	assert.Len(t, r.ranges, 2)
	r.Add(FileRange{Start: 15, End: 20})
	assert.Len(t, r.ranges, 1)
}
