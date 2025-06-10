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
		assert.Equal(t, uint64(10), r.StartOffset())
		assert.Equal(t, uint64(20), r.EndOffset())
		assert.Equal(t, uint64(0), r.seq)
		r.Set(5, 30)
		assert.Equal(t, uint64(1), r.seq)
	})
	t.Run("SetGet/Int64", func(t *testing.T) {
		var r Range
		r.SetInt64(10, 20)
		assert.Equal(t, int64(10), r.StartOffsetInt64())
		assert.Equal(t, int64(20), r.EndOffsetInt64())
		r.SetInt64(-1, 0)
		assert.Equal(t, int64(10), r.StartOffsetInt64())
		assert.Equal(t, int64(20), r.EndOffsetInt64())
		r.Set(5, math.MaxUint64)
		assert.Equal(t, int64(5), r.StartOffsetInt64())
		assert.Equal(t, int64(0), r.EndOffsetInt64())
	})
	t.Run("Shift", func(t *testing.T) {
		var r Range
		r.ShiftInt64(100)
		assert.Equal(t, uint64(0), r.start)
		assert.Equal(t, uint64(100), r.end)
		assert.Equal(t, uint64(0), r.seq)
		r.ShiftInt64(200)
		assert.Equal(t, uint64(100), r.start)
		assert.Equal(t, uint64(200), r.end)
		assert.Equal(t, uint64(0), r.seq)
		r.ShiftInt64(50)
		assert.Equal(t, uint64(0), r.start)
		assert.Equal(t, uint64(50), r.end)
		assert.Equal(t, uint64(1), r.seq)
		r.ShiftInt64(-1)
		assert.Equal(t, uint64(0), r.start)
		assert.Equal(t, uint64(50), r.end)
		assert.Equal(t, uint64(1), r.seq)
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

func TestRanges(t *testing.T) {
	t.Run("Last", func(t *testing.T) {
		assert.Equal(t, Range{}, RangeList{}.Last())
		assert.Equal(t, Range{start: 20, end: 40}, RangeList{
			Range{start: 0, end: 10},
			Range{start: 20, end: 40},
		}.Last())
	})
	t.Run("OnlyUseMaxOffset", func(t *testing.T) {
		r := RangeList{}
		assert.True(t, r.OnlyUseMaxOffset())
		assert.EqualValues(t, 0, r.Last().EndOffset())
		r = RangeList{
			Range{start: 0, end: 10},
		}
		assert.True(t, r.OnlyUseMaxOffset())
		assert.EqualValues(t, 10, r.Last().EndOffset())
		r = RangeList{
			Range{start: 10, end: 20},
		}
		assert.False(t, r.OnlyUseMaxOffset())
		r = RangeList{
			Range{start: 0, end: 20},
			Range{start: 30, end: 40},
		}
		assert.False(t, r.OnlyUseMaxOffset())
	})
}

func TestNewRangeTracker(t *testing.T) {
	for i := -10; i <= 10; i++ {
		tracker := newRangeTracker("test", i)
		if i <= 1 {
			_, ok := tracker.(*singleRangeTracker)
			assert.True(t, ok)
		} else {
			mrt, ok := tracker.(*multiRangeTracker)
			assert.True(t, ok)
			assert.EqualValues(t, i, mrt.cap)
		}
	}
}

func TestSingleRangeTracker_Insert(t *testing.T) {
	t.Run("NonOverlapping", func(t *testing.T) {
		t.Parallel()
		tracker := newSingleRangeTracker("test")
		assert.True(t, tracker.Insert(Range{start: 0, end: 5}))
		assert.True(t, tracker.Insert(Range{start: 20, end: 30}))
		assert.Equal(t, 1, tracker.Len())
		assert.Equal(t, "[0-30]", tracker.Ranges().String())
		assert.True(t, tracker.Insert(Range{start: 5, end: 15, seq: 1}))
		assert.Equal(t, 1, tracker.Len())
		assert.Equal(t, "[0-15]", tracker.Ranges().String())
	})
	t.Run("AlreadyContained", func(t *testing.T) {
		t.Parallel()
		tracker := newSingleRangeTracker("test")
		assert.True(t, tracker.Insert(Range{start: 0, end: 20}))
		assert.False(t, tracker.Insert(Range{start: 10, end: 15}))
		assert.False(t, tracker.Insert(Range{start: 0, end: 20}))
		assert.Equal(t, 1, tracker.Len())
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		tracker := newSingleRangeTracker("test")
		r := Range{start: 0, end: 0}
		assert.False(t, r.IsValid())
		assert.False(t, tracker.Insert(r))
		r.Set(10, 5)
		assert.False(t, tracker.Insert(r))
	})
}

func TestSingleRangeTracker_Unmarshal(t *testing.T) {
	t.Run("Marshal/Loop", func(t *testing.T) {
		want := RangeList{
			{start: 0, end: 50},
		}
		tracker := newSingleRangeTracker("test")
		assert.NoError(t, tracker.UnmarshalText([]byte("50\ntest\n0-5,20-30,45-50")))
		assert.Equal(t, 1, tracker.Len())
		assert.Equal(t, want, tracker.Ranges())
		got, err := tracker.MarshalText()
		assert.NoError(t, err)
		assert.Equal(t, "50\ntest\n0-50", string(got))
		assert.NoError(t, tracker.UnmarshalText(got))
		assert.Equal(t, want, tracker.Ranges())
		tracker.Clear()
		assert.Equal(t, 0, tracker.Len())
		got, err = tracker.MarshalText()
		assert.NoError(t, err)
		assert.Equal(t, "0\ntest", string(got))
	})
	t.Run("Empty", func(t *testing.T) {
		tracker := newSingleRangeTracker("test")
		assert.NoError(t, tracker.UnmarshalText([]byte("")))
		assert.Equal(t, 0, tracker.Len())
	})
	t.Run("Invalid/SingleLine", func(t *testing.T) {
		tracker := newSingleRangeTracker("test")
		assert.Error(t, tracker.UnmarshalText([]byte("test")))
		assert.Equal(t, 0, tracker.Len())
	})
	t.Run("Invalid/MultiLine", func(t *testing.T) {
		tracker := newSingleRangeTracker("test")
		assert.Error(t, tracker.UnmarshalText([]byte("test\ntest\ntest")))
		assert.Equal(t, 0, tracker.Len())
	})
	t.Run("Invalid/MissingMaxOffset", func(t *testing.T) {
		tracker := newSingleRangeTracker("test")
		assert.Error(t, tracker.UnmarshalText([]byte("0-15,20-30\ntest")))
		assert.Equal(t, 0, tracker.Len())
	})
	t.Run("Invalid/Range", func(t *testing.T) {
		tracker := newSingleRangeTracker("test")
		assert.NoError(t, tracker.UnmarshalText([]byte("50\ntest-test\ntest")))
		assert.Equal(t, RangeList{
			{start: 0, end: 50},
		}, tracker.Ranges())
	})
	t.Run("Invalid/OutOfOrder", func(t *testing.T) {
		tracker := newSingleRangeTracker("test")
		assert.NoError(t, tracker.UnmarshalText([]byte("50\n10-20,30-50\ntest")))
		assert.Equal(t, RangeList{
			{start: 0, end: 50},
		}, tracker.Ranges())
	})
	t.Run("BackwardsCompatible/Invalid", func(t *testing.T) {
		tracker := newSingleRangeTracker("test")
		assert.Error(t, tracker.UnmarshalText([]byte("-1\ntest")))
		assert.Equal(t, 0, tracker.Len())
	})
	t.Run("BackwardsCompatible/Valid", func(t *testing.T) {
		tracker := newSingleRangeTracker("test")
		assert.NoError(t, tracker.UnmarshalText([]byte("20")))
		assert.Equal(t, RangeList{
			{start: 0, end: 20},
		}, tracker.Ranges())
		assert.NoError(t, tracker.UnmarshalText([]byte("50\ntest")))
		assert.Equal(t, RangeList{
			{start: 0, end: 50},
		}, tracker.Ranges())
	})
}

func TestMultiRangeTracker_Insert(t *testing.T) {
	t.Run("NonOverlapping", func(t *testing.T) {
		t.Parallel()
		tracker := newMultiRangeTracker("test", -1)
		assert.True(t, tracker.Insert(Range{start: 0, end: 5}))
		assert.True(t, tracker.Insert(Range{start: 20, end: 30}))
		assert.Equal(t, 2, tracker.Len())
		assert.Equal(t, "[0-5,20-30]", tracker.Ranges().String())
		assert.False(t, tracker.Insert(Range{start: 20, end: 30, seq: 1}))
		assert.Equal(t, 2, tracker.Len())
		assert.Equal(t, "[0-5,20-30]", tracker.Ranges().String())
	})
	t.Run("Merge/Adjacent", func(t *testing.T) {
		t.Parallel()
		tracker := newMultiRangeTracker("test", -1)
		assert.True(t, tracker.Insert(Range{start: 0, end: 5}))
		assert.True(t, tracker.Insert(Range{start: 20, end: 30}))
		assert.True(t, tracker.Insert(Range{start: 5, end: 10}))
		assert.Equal(t, 2, tracker.Len())
		assert.Equal(t, "[0-10,20-30]", tracker.Ranges().String())
		assert.True(t, tracker.Insert(Range{start: 15, end: 20}))
		assert.Equal(t, 2, tracker.Len())
		assert.Equal(t, "[0-10,15-30]", tracker.Ranges().String())
	})
	t.Run("Merge/Overlap/Single", func(t *testing.T) {
		t.Parallel()
		tracker := newMultiRangeTracker("test", -1)
		assert.True(t, tracker.Insert(Range{start: 0, end: 5}))
		assert.True(t, tracker.Insert(Range{start: 20, end: 30}))
		assert.True(t, tracker.Insert(Range{start: 15, end: 25}))
		assert.Equal(t, 2, tracker.Len())
		assert.Equal(t, "[0-5,15-30]", tracker.Ranges().String())
		assert.True(t, tracker.Insert(Range{start: 2, end: 8}))
		assert.Equal(t, 2, tracker.Len())
		assert.Equal(t, "[0-8,15-30]", tracker.Ranges().String())
	})
	t.Run("Merge/Overlap/Multiple", func(t *testing.T) {
		t.Parallel()
		tracker := newMultiRangeTracker("test", -1)
		assert.True(t, tracker.Insert(Range{start: 0, end: 5}))
		assert.True(t, tracker.Insert(Range{start: 20, end: 30}))
		assert.True(t, tracker.Insert(Range{start: 10, end: 15}))
		assert.Equal(t, 3, tracker.Len())
		assert.Equal(t, "[0-5,10-15,20-30]", tracker.Ranges().String())
		assert.True(t, tracker.Insert(Range{start: 5, end: 25}))
		assert.Equal(t, 1, tracker.Len())
		assert.Equal(t, "[0-30]", tracker.Ranges().String())
	})
	t.Run("AlreadyContained", func(t *testing.T) {
		t.Parallel()
		tracker := newMultiRangeTracker("test", -1)
		assert.True(t, tracker.Insert(Range{start: 0, end: 20}))
		assert.False(t, tracker.Insert(Range{start: 10, end: 15}))
		assert.False(t, tracker.Insert(Range{start: 0, end: 20}))
		assert.Equal(t, 1, tracker.Len())
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		tracker := newMultiRangeTracker("test", -1)
		r := Range{start: 0, end: 0}
		assert.False(t, r.IsValid())
		assert.False(t, tracker.Insert(r))
		r.Set(10, 5)
		assert.False(t, tracker.Insert(r))
	})
	t.Run("CollapseWhenOverCapacity", func(t *testing.T) {
		t.Parallel()
		tracker := newMultiRangeTracker("test", 2)
		assert.True(t, tracker.Insert(Range{start: 0, end: 5}))
		assert.True(t, tracker.Insert(Range{start: 10, end: 15}))
		assert.True(t, tracker.Insert(Range{start: 20, end: 25}))
		assert.Equal(t, 2, tracker.Len())
		assert.Equal(t, "[0-15,20-25]", tracker.Ranges().String())
	})
}

func TestMultiRangeTracker_Unmarshal(t *testing.T) {
	t.Run("Marshal/Loop", func(t *testing.T) {
		want := RangeList{
			{start: 0, end: 5},
			{start: 20, end: 30},
			{start: 45, end: 50},
		}
		tracker := newMultiRangeTracker("test", -1)
		assert.NoError(t, tracker.UnmarshalText([]byte("50\ntest\n0-5,20-30,45-50")))
		assert.Equal(t, 3, tracker.Len())
		assert.Equal(t, want, tracker.Ranges())
		got, err := tracker.MarshalText()
		assert.NoError(t, err)
		assert.Equal(t, "50\ntest\n0-5,20-30,45-50", string(got))
		tracker.Clear()
		assert.Equal(t, 0, tracker.Len())
		assert.NoError(t, tracker.UnmarshalText(got))
		assert.Equal(t, want, tracker.Ranges())
		tracker.Clear()
		assert.Equal(t, 0, tracker.Len())
		got, err = tracker.MarshalText()
		assert.NoError(t, err)
		assert.Equal(t, "0\ntest", string(got))
	})
	t.Run("Empty", func(t *testing.T) {
		tracker := newMultiRangeTracker("test", -1)
		assert.NoError(t, tracker.UnmarshalText([]byte("")))
		assert.Equal(t, 0, tracker.Len())
	})
	t.Run("Invalid/SingleLine", func(t *testing.T) {
		tracker := newMultiRangeTracker("test", -1)
		assert.Error(t, tracker.UnmarshalText([]byte("test")))
		assert.Equal(t, 0, tracker.Len())
	})
	t.Run("Invalid/MultiLine", func(t *testing.T) {
		tracker := newMultiRangeTracker("test", -1)
		assert.Error(t, tracker.UnmarshalText([]byte("test\ntest\ntest")))
		assert.Equal(t, 0, tracker.Len())
	})
	t.Run("Invalid/MissingMaxOffset", func(t *testing.T) {
		tracker := newMultiRangeTracker("test", -1)
		assert.Error(t, tracker.UnmarshalText([]byte("0-15,20-30\ntest")))
		assert.Equal(t, 0, tracker.Len())
	})
	t.Run("Invalid/Range", func(t *testing.T) {
		tracker := newMultiRangeTracker("test", -1)
		assert.NoError(t, tracker.UnmarshalText([]byte("50\ntest-test\ntest")))
		assert.Equal(t, RangeList{
			{start: 0, end: 50},
		}, tracker.Ranges())
	})
	t.Run("Invalid/OutOfOrder", func(t *testing.T) {
		tracker := newMultiRangeTracker("test", -1)
		assert.NoError(t, tracker.UnmarshalText([]byte("50\n10-20,30-50\ntest")))
		assert.Equal(t, RangeList{
			{start: 0, end: 50},
		}, tracker.Ranges())
	})
	t.Run("BackwardsCompatible/Invalid", func(t *testing.T) {
		tracker := newMultiRangeTracker("test", -1)
		assert.Error(t, tracker.UnmarshalText([]byte("-1\ntest")))
		assert.Equal(t, 0, tracker.Len())
	})
	t.Run("BackwardsCompatible/Valid", func(t *testing.T) {
		tracker := newMultiRangeTracker("test", -1)
		assert.NoError(t, tracker.UnmarshalText([]byte("20")))
		assert.Equal(t, RangeList{
			{start: 0, end: 20},
		}, tracker.Ranges())
		assert.NoError(t, tracker.UnmarshalText([]byte("50\ntest")))
		assert.Equal(t, RangeList{
			{start: 0, end: 50},
		}, tracker.Ranges())
	})
}

func TestMultiRangeTracker_Ranges(t *testing.T) {
	tracker := newMultiRangeTracker("test", -1)
	got := tracker.Ranges()
	assert.NotNil(t, got)
	assert.Empty(t, got)
	tracker.Insert(Range{start: 0, end: 5})
	got = tracker.Ranges()
	assert.Equal(t, RangeList{
		Range{start: 0, end: 5},
	}, got)
}

func TestInvertRanges(t *testing.T) {
	tracker := newMultiRangeTracker("test", -1)
	assert.True(t, tracker.Insert(Range{start: 5, end: 10}))
	assert.True(t, tracker.Insert(Range{start: 20, end: 25}))
	ranges := tracker.Ranges()
	got := InvertRanges(ranges)
	want := RangeList{
		{start: 0, end: 5},
		{start: 10, end: 20},
		{start: 25, end: unboundedEnd},
	}
	assert.Equal(t, want, got)
	tracker.Clear()
	want = RangeList{
		{start: 0, end: unboundedEnd},
	}
	assert.Equal(t, want, InvertRanges(tracker.Ranges()))
}

func BenchmarkMultiRangeTracker(b *testing.B) {
	b.Run("Insert", func(b *testing.B) {
		tracker := newMultiRangeTracker("test", 50)
		r := rand.New(rand.NewSource(64))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := r.Uint64() % 1000000
			end := start + uint64(r.Intn(100)+1)
			tracker.Insert(Range{start: start, end: end})
		}
	})
	b.Run("Insert/NonOverlapping", func(b *testing.B) {
		tracker := newMultiRangeTracker("test", 50)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tracker.Insert(Range{start: uint64(i * 10), end: uint64(i*10 + 5)})
		}
	})
	b.Run("Invert", func(b *testing.B) {
		tracker := newMultiRangeTracker("test", 50)
		for i := 0; i < b.N; i++ {
			tracker.Insert(Range{start: uint64(i * 10), end: uint64(i*10 + 5)})
		}
		ranges := tracker.Ranges()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = InvertRanges(ranges)
		}
	})
	b.Run("Ranges", func(b *testing.B) {
		tracker := newMultiRangeTracker("test", 1000)
		for i := 0; i < b.N; i++ {
			tracker.Insert(Range{start: uint64(i * 10), end: uint64(i*10 + 5)})
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = tracker.Ranges()
		}
	})
}
