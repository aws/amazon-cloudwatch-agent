// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRangeTree_Insert(t *testing.T) {
	tree := NewRangeTree()
	assert.True(t, tree.Insert(Range{start: 0, end: 5}))
	assert.True(t, tree.Insert(Range{start: 20, end: 30}))
	assert.Equal(t, 2, tree.tree.Len())
	assert.Equal(t, "[0-5,20-30]", tree.String())
	// merge continuous
	assert.True(t, tree.Insert(Range{start: 5, end: 10}))
	assert.Equal(t, 2, tree.tree.Len())
	assert.Equal(t, "[0-10,20-30]", tree.String())
	// merge overlap
	assert.True(t, tree.Insert(Range{start: 15, end: 25}))
	assert.Equal(t, 2, tree.tree.Len())
	assert.Equal(t, "[0-10,15-30]", tree.String())
	// fully contained
	assert.False(t, tree.Insert(Range{start: 0, end: 10}))
	assert.Equal(t, 2, tree.tree.Len())
	assert.Equal(t, "[0-10,15-30]", tree.String())
	// invalid range
	assert.False(t, tree.Insert(Range{start: 10, end: 10}))
	assert.Equal(t, 2, tree.tree.Len())
	assert.Equal(t, "[0-10,15-30]", tree.String())
	// combine
	assert.True(t, tree.Insert(Range{start: 10, end: 15}))
	assert.Equal(t, 1, tree.tree.Len())
	assert.Equal(t, "[0-30]", tree.String())
}

func TestRangeTree_UnmarshalText(t *testing.T) {
	tree := NewRangeTree()
	assert.NoError(t, tree.UnmarshalText([]byte("50\n0-5,20-30,45-50\ntest")))
	assert.Equal(t, 3, tree.tree.Len())
	got, err := tree.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, "50\n0-5,20-30,45-50", string(got))

	tree = NewRangeTree()
	assert.Error(t, tree.UnmarshalText([]byte("0-5,20-30,45-50\ntest")))
	assert.Equal(t, 0, tree.tree.Len())
	got, err = tree.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, "0\n", string(got))

	tree = NewRangeTree()
	assert.Error(t, tree.UnmarshalText([]byte("50\ntest")))
	assert.Equal(t, 1, tree.tree.Len())
	got, err = tree.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, "50\n0-50", string(got))
}

func TestRangeTree_Gaps(t *testing.T) {
	tree := NewRangeTree()
	assert.True(t, tree.Insert(Range{start: 0, end: 10}))
	assert.True(t, tree.Insert(Range{start: 20, end: 25}))
	gaps := Gaps(tree.Ranges())
	expected := []Range{
		{start: 10, end: 20},
		{start: 25, end: math.MaxUint64},
	}
	assert.Equal(t, expected, gaps)
}
