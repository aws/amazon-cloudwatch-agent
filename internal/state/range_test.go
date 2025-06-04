// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//nolint:gosec
package state

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRange(t *testing.T) {
	t.Run("SetGet", func(t *testing.T) {
		var r Range
		r.Set(10, 20)
		start, end := r.Get()
		assert.Equal(t, uint64(10), start)
		assert.Equal(t, uint64(20), end)
		assert.Equal(t, uint64(0), r.seq)
		r.Set(5, 30)
		assert.Equal(t, uint64(1), r.seq)
	})
	t.Run("SetGet/Int64", func(t *testing.T) {
		var r Range
		r.SetInt64(10, 20)
		start, end := r.GetInt64()
		assert.Equal(t, int64(10), start)
		assert.Equal(t, int64(20), end)
		r.SetInt64(-1, 0)
		start, end = r.GetInt64()
		assert.Equal(t, int64(10), start)
		assert.Equal(t, int64(20), end)
		r.Set(5, math.MaxUint64)
		start, end = r.GetInt64()
		assert.Equal(t, int64(5), start)
		assert.Equal(t, int64(0), end)
	})
	t.Run("Contains", func(t *testing.T) {
		r1 := Range{start: 0, end: 10}
		r2 := Range{start: 3, end: 7}
		r3 := Range{start: 5, end: 15}
		assert.True(t, r1.Contains(r2))
		assert.False(t, r2.Contains(r1))
		assert.False(t, r1.Contains(r3))
		assert.True(t, r1.Contains(r1))
	})
	t.Run("Unmarshal", func(t *testing.T) {
		r := Range{start: 5, end: 10}
		got, err := r.MarshalText()
		assert.NoError(t, err)
		assert.Equal(t, []byte("5-10"), got)

		var r2 Range
		assert.NoError(t, r2.UnmarshalText(got))
		assert.Equal(t, r, r2)
	})
	t.Run("Unmarshal/Unbounded", func(t *testing.T) {
		r := Range{start: 5, end: unboundedEnd}
		got, err := r.MarshalText()
		assert.NoError(t, err)
		assert.Equal(t, []byte("5-"), got)

		var r2 Range
		assert.NoError(t, r2.UnmarshalText(got))
		assert.Equal(t, r, r2)
	})
	t.Run("Unmarshal/Invalid", func(t *testing.T) {
		testCases := []string{
			"invalid",
			"5",
			"5-a",
			"a-5",
			"5-5",
			"5-5-10",
		}
		for _, testCase := range testCases {
			var r Range
			assert.Error(t, r.UnmarshalText([]byte(testCase)))
			assert.Equal(t, Range{}, r)
		}
	})
}

func TestRangeTree_Insert(t *testing.T) {
	t.Run("NonOverlapping", func(t *testing.T) {
		t.Parallel()
		tree := NewRangeTree()
		assert.True(t, tree.Insert(Range{start: 0, end: 5}))
		assert.True(t, tree.Insert(Range{start: 20, end: 30}))
		assert.Equal(t, 2, tree.len())
		assert.Equal(t, "[0-5,20-30]", tree.String())
		assert.False(t, tree.Insert(Range{start: 20, end: 30, seq: 1}))
		assert.Equal(t, 2, tree.len())
		assert.Equal(t, "[0-5,20-30]", tree.String())
	})
	t.Run("Merge/Adjacent", func(t *testing.T) {
		t.Parallel()
		tree := NewRangeTree()
		assert.True(t, tree.Insert(Range{start: 0, end: 5}))
		assert.True(t, tree.Insert(Range{start: 20, end: 30}))
		assert.True(t, tree.Insert(Range{start: 5, end: 10}))
		assert.Equal(t, 2, tree.len())
		assert.Equal(t, "[0-10,20-30]", tree.String())
		assert.True(t, tree.Insert(Range{start: 15, end: 20}))
		assert.Equal(t, 2, tree.len())
		assert.Equal(t, "[0-10,15-30]", tree.String())
	})
	t.Run("Merge/Overlap/Single", func(t *testing.T) {
		t.Parallel()
		tree := NewRangeTree()
		assert.True(t, tree.Insert(Range{start: 0, end: 5}))
		assert.True(t, tree.Insert(Range{start: 20, end: 30}))
		assert.True(t, tree.Insert(Range{start: 15, end: 25}))
		assert.Equal(t, 2, tree.len())
		assert.Equal(t, "[0-5,15-30]", tree.String())
		assert.True(t, tree.Insert(Range{start: 2, end: 8}))
		assert.Equal(t, 2, tree.len())
		assert.Equal(t, "[0-8,15-30]", tree.String())
	})
	t.Run("Merge/Overlap/Multiple", func(t *testing.T) {
		t.Parallel()
		tree := NewRangeTree()
		assert.True(t, tree.Insert(Range{start: 0, end: 5}))
		assert.True(t, tree.Insert(Range{start: 20, end: 30}))
		assert.True(t, tree.Insert(Range{start: 10, end: 15}))
		assert.Equal(t, 3, tree.len())
		assert.Equal(t, "[0-5,10-15,20-30]", tree.String())
		assert.True(t, tree.Insert(Range{start: 5, end: 25}))
		assert.Equal(t, 1, tree.len())
		assert.Equal(t, "[0-30]", tree.String())
	})
	t.Run("AlreadyContained", func(t *testing.T) {
		t.Parallel()
		tree := NewRangeTree()
		assert.True(t, tree.Insert(Range{start: 0, end: 20}))
		assert.False(t, tree.Insert(Range{start: 10, end: 15}))
		assert.False(t, tree.Insert(Range{start: 0, end: 20}))
		assert.Equal(t, 1, tree.len())
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		tree := NewRangeTree()
		r := Range{start: 0, end: 0}
		assert.False(t, r.IsValid())
		assert.False(t, tree.Insert(r))
		r.Set(10, 5)
		assert.False(t, tree.Insert(r))
	})
	t.Run("CollapseWhenOverCapacity", func(t *testing.T) {
		t.Parallel()
		tree := NewRangeTreeWithCap(2)
		assert.True(t, tree.Insert(Range{start: 0, end: 5}))
		assert.True(t, tree.Insert(Range{start: 10, end: 15}))
		assert.True(t, tree.Insert(Range{start: 20, end: 25}))
		assert.Equal(t, 2, tree.len())
		assert.Equal(t, "[0-15,20-25]", tree.String())
	})
}

func TestRangeTree_Unmarshal(t *testing.T) {
	t.Run("MarshalLoop", func(t *testing.T) {
		tree := NewRangeTree()
		assert.NoError(t, tree.UnmarshalText([]byte("50\n0-5,20-30,45-50\ntest")))
		assert.Equal(t, 3, tree.len())
		assert.Equal(t, []Range{
			{start: 0, end: 5},
			{start: 20, end: 30},
			{start: 45, end: 50},
		}, tree.Ranges())
		got, err := tree.MarshalText()
		assert.NoError(t, err)
		assert.Equal(t, "50\n0-5,20-30,45-50", string(got))
	})
	t.Run("Empty", func(t *testing.T) {
		tree := NewRangeTree()
		assert.NoError(t, tree.UnmarshalText([]byte("")))
		assert.Len(t, tree.Ranges(), 0)
	})
	t.Run("Invalid", func(t *testing.T) {
		tree := NewRangeTree()
		assert.Error(t, tree.UnmarshalText([]byte("test\ntest\ntest")))
		assert.Len(t, tree.Ranges(), 0)
	})
	t.Run("Invalid/MissingMaxOffset", func(t *testing.T) {
		tree := NewRangeTree()
		assert.Error(t, tree.UnmarshalText([]byte("0-15,20-30\ntest")))
		assert.Len(t, tree.Ranges(), 0)
	})
	t.Run("BackwardsCompatible/Invalid", func(t *testing.T) {
		tree := NewRangeTree()
		assert.Error(t, tree.UnmarshalText([]byte("-1\ntest")))
		assert.Len(t, tree.Ranges(), 0)
	})
	t.Run("BackwardsCompatible/Valid", func(t *testing.T) {
		tree := NewRangeTree()
		assert.Error(t, tree.UnmarshalText([]byte("50\ntest")))
		assert.Equal(t, []Range{
			{start: 0, end: 50},
		}, tree.Ranges())
	})
}

func TestRangeTree_Invert(t *testing.T) {
	tree := NewRangeTree()
	assert.True(t, tree.Insert(Range{start: 5, end: 10}))
	assert.True(t, tree.Insert(Range{start: 20, end: 25}))
	ranges := tree.Ranges()
	inverted := Invert(ranges)
	want := []Range{
		{start: 0, end: 5},
		{start: 10, end: 20},
		{start: 25, end: unboundedEnd},
	}
	assert.Equal(t, want, inverted)
	inverted = Invert(want)
	assert.Equal(t, ranges, inverted)
}

func BenchmarkRangeTree(b *testing.B) {
	b.Run("Insert", func(b *testing.B) {
		tree := NewRangeTreeWithCap(50)
		r := rand.New(rand.NewSource(64))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := r.Uint64() % 1000000
			end := start + uint64(r.Intn(100)+1)
			tree.Insert(Range{start: start, end: end})
		}
	})
	b.Run("Insert/NonOverlapping", func(b *testing.B) {
		tree := NewRangeTreeWithCap(50)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tree.Insert(Range{start: uint64(i * 10), end: uint64(i*10 + 5)})
		}
	})
	b.Run("Invert", func(b *testing.B) {
		tree := NewRangeTreeWithCap(50)
		for i := 0; i < b.N; i++ {
			tree.Insert(Range{start: uint64(i * 10), end: uint64(i*10 + 5)})
		}
		ranges := tree.Ranges()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = Invert(ranges)
		}
	})
	b.Run("Ranges", func(b *testing.B) {
		tree := NewRangeTreeWithCap(1000)
		for i := 0; i < b.N; i++ {
			tree.Insert(Range{start: uint64(i * 10), end: uint64(i*10 + 5)})
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = tree.Ranges()
		}
	})
}
