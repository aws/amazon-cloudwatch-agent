// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"bytes"
	"encoding"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/google/btree"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

const (
	defaultBTreeDegree = 2
	unboundedEnd       = math.MaxUint64
)

// Range represents a pair of offsets [start, end).
type Range struct {
	start, end uint64
	// seq handles file truncation, when a file is truncated, we increase the seq
	seq uint64
}

var _ encoding.TextMarshaler = (*Range)(nil)
var _ encoding.TextUnmarshaler = (*Range)(nil)

// Set updates the start and end offsets of the range. If the new start is before the current start, it indicates
// file truncation and increments the sequence number.
func (r *Range) Set(start, end uint64) {
	if start < r.start {
		r.seq++
	}
	r.start, r.end = start, end
}

// SetInt64 is the int64 version of Set. If start or end are negative, the range is not updated.
func (r *Range) SetInt64(start, end int64) {
	if start < 0 || end < 0 {
		return
	}
	r.Set(uint64(start), uint64(end))
}

// StartOffset returns the inclusive start of the range.
func (r Range) StartOffset() uint64 {
	return r.start
}

// EndOffset returns the exclusive end of the range.
func (r Range) EndOffset() uint64 {
	return r.end
}

// StartOffsetInt64 is the int64 version of StartOffset. If start exceeds math.MaxInt64, returns 0.
func (r Range) StartOffsetInt64() int64 {
	return convertInt64(r.start)
}

// EndOffsetInt64 is the int64 version of EndOffset. If end exceeds math.MaxInt64, returns 0.
func (r Range) EndOffsetInt64() int64 {
	return convertInt64(r.end)
}

// Shift moves the previous end to the start and sets the new end. If the new end is before the previous one, it resets
// the range to [0, newEnd) and increments the sequence number.
func (r *Range) Shift(newEnd uint64) {
	if newEnd < r.end {
		r.seq++
		r.start, r.end = 0, newEnd
	} else {
		r.start = r.end
		r.end = newEnd
	}
}

// ShiftInt64 is the int64 version of Shift. If newEnd is negative, the range is not updated.
func (r *Range) ShiftInt64(newEnd int64) {
	if newEnd < 0 {
		return
	}
	r.Shift(uint64(newEnd))
}

// IsEndOffsetUnbounded returns true if the end offset is the unbounded representation (i.e. math.MaxUint64).
func (r Range) IsEndOffsetUnbounded() bool {
	return r.end == unboundedEnd
}

// IsValid returns true if the range is ordered (i.e. start < end).
func (r Range) IsValid() bool {
	return r.start < r.end
}

// Contains returns true if the range completely contains the other range.
func (r Range) Contains(other Range) bool {
	return r.start <= other.start && r.end >= other.end
}

// String returns a string representation of the range "start-end". If the end is unbounded, returns "start-".
func (r Range) String() string {
	if r.IsEndOffsetUnbounded() {
		return fmt.Sprintf("%v-", r.start)
	}
	return fmt.Sprintf("%d-%d", r.start, r.end)
}

func (r Range) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

// UnmarshalText supports unmarshalling both the "start-end" and "start-" formats.
func (r *Range) UnmarshalText(text []byte) error {
	parts := strings.SplitN(string(text), "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid format: %q", text)
	}
	start, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid start: %s", parts[0])
	}
	var end uint64
	if parts[1] == "" {
		end = unboundedEnd
	} else {
		end, err = strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid end: %s", parts[1])
		}
	}
	tmp := Range{start: start, end: end}
	if !tmp.IsValid() {
		return fmt.Errorf("invalid range: %q", text)
	}
	*r = tmp
	return nil
}

// RangeList is a slice of Range values.
type RangeList []Range

// Last returns the last Range in the slice. If empty, returns the zero-value Range.
func (r RangeList) Last() Range {
	if len(r) == 0 {
		return Range{}
	}
	return r[len(r)-1]
}

// OnlyUseMaxOffset returns true if the RangeList is either empty or only contains a single Range that starts at 0.
// The intention of this is to maintain backwards compatibility with state files that only store the offset.
func (r RangeList) OnlyUseMaxOffset() bool {
	return len(r) == 0 || (len(r) == 1 && r[0].StartOffset() == 0)
}

// rangeTree is used to store and merge ranges. It is not thread-safe.
type rangeTree struct {
	// cap is the maximum number of ranges that can be in the tree.
	cap int
	// tree is the backing B-tree.
	tree *btree.BTreeG[Range]
}

var _ encoding.TextMarshaler = (*rangeTree)(nil)
var _ encoding.TextUnmarshaler = (*rangeTree)(nil)

// newRangeTree creates an unbounded rangeTree.
func newRangeTree() *rangeTree {
	return newRangeTreeWithCap(-1)
}

// newRangeTreeWithCap creates a bounded rangeTree based on the capacity. When capacity is exceeded, the oldest ranges
// are merged/collapsed.
func newRangeTreeWithCap(capacity int) *rangeTree {
	return &rangeTree{
		cap:  capacity,
		tree: btree.NewG(defaultBTreeDegree, lessRange),
	}
}

// Insert a Range into the tree. Returns false if the Range is already contained by another Range in the tree or is
// invalid.
//
// If the added Range overlaps or is adjacent with any existing Range, they are merged. If the tree capacity is hit
// after the insert, merges the two bottom ranges.
func (t *rangeTree) Insert(r Range) bool {
	if !r.IsValid() {
		return false
	}
	toRemove := collections.NewSet[Range]()
	merged := r
	var contained bool
	t.tree.AscendGreaterOrEqual(r, func(item Range) bool {
		if item.start > merged.end {
			return false
		}
		if item.Contains(r) {
			contained = true
			return false
		}
		if shouldMerge(item, merged) {
			toRemove.Add(item)
			merged = mergeRanges(merged, item)
		}
		return true
	})
	if contained {
		return false
	}
	t.tree.DescendLessOrEqual(r, func(item Range) bool {
		if item.end < merged.start {
			return false
		}
		if item.Contains(r) {
			contained = true
			return false
		}
		if shouldMerge(item, merged) {
			toRemove.Add(item)
			merged = mergeRanges(merged, item)
		}
		return true
	})
	if contained {
		return false
	}
	for item := range toRemove {
		t.tree.Delete(item)
	}
	t.tree.ReplaceOrInsert(merged)
	if t.cap > 0 && t.tree.Len() > t.cap {
		t.collapseOldest()
	}
	return true
}

// collapseOldest takes the two oldest (i.e. smallest start) ranges and merges them.
func (t *rangeTree) collapseOldest() {
	var first, second *Range
	var count int
	t.tree.Ascend(func(item Range) bool {
		if count == 0 {
			first = new(Range)
			*first = item
		} else if count == 1 {
			second = new(Range)
			*second = item
			return false
		}
		count++
		return true
	})
	if first == nil || second == nil {
		return
	}
	t.tree.Delete(*first)
	t.tree.Delete(*second)

	merged := mergeRanges(*first, *second)
	t.tree.ReplaceOrInsert(merged)
}

// Ranges returns all ranges in sorted order.
func (t *rangeTree) Ranges() RangeList {
	ranges := make(RangeList, 0, t.tree.Len())
	t.tree.Ascend(func(item Range) bool {
		ranges = append(ranges, item)
		return true
	})
	return ranges
}

// Len the number of ranges in the tree.
func (t *rangeTree) Len() int {
	return t.tree.Len()
}

// Clear removes all ranges in the tree.
func (t *rangeTree) Clear() {
	t.tree.Clear(false)
}

// String returns a string representation of all stored ranges (e.g. "[0-5,10-15]").
func (t *rangeTree) String() string {
	var builder strings.Builder
	first := true
	builder.WriteByte('[')
	t.tree.Ascend(func(item Range) bool {
		if !first {
			builder.WriteByte(',')
		}
		first = false
		builder.WriteString(item.String())
		return true
	})
	builder.WriteByte(']')
	return builder.String()
}

// MarshalText serializes the tree. The format includes the maximum offset on the first line (for backwards
// compatibility) followed by comma-separated ranges (e.g. "5\n0-5")
func (t *rangeTree) MarshalText() ([]byte, error) {
	var rangeBuf bytes.Buffer
	var maxEnd uint64
	first := true
	var err error

	t.tree.Ascend(func(item Range) bool {
		if item.end > maxEnd {
			maxEnd = item.end
		}
		if !first {
			rangeBuf.WriteByte(',')
		}
		first = false
		var text []byte
		text, err = item.MarshalText()
		if err != nil {
			return false
		}
		rangeBuf.Write(text)
		return true
	})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	_, err = fmt.Fprintf(&buf, "%d\n", maxEnd)
	if err != nil {
		return nil, err
	}
	_, err = buf.Write(rangeBuf.Bytes())
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalText deserializes the text to populate the tree. For backwards compatibility, if there is a maximum offset
// but no ranges, populates the tree with [0, maxOffset).
func (t *rangeTree) UnmarshalText(text []byte) error {
	t.Clear()
	if len(text) == 0 {
		return nil
	}
	lines := bytes.Split(text, []byte("\n"))
	maxOffset, err := strconv.ParseUint(string(lines[0]), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid max offset: %w", err)
	}
	defer func() {
		if t.tree.Len() == 0 {
			t.Insert(Range{start: 0, end: maxOffset})
		}
	}()
	if len(lines) < 2 {
		return nil
	}
	parts := bytes.Split(lines[1], []byte(","))
	for _, part := range parts {
		var r Range
		if err = r.UnmarshalText(part); err != nil {
			// clear any inserted ranges and fallback to offset only case
			t.Clear()
			break
		}
		t.Insert(r)
	}
	return nil
}

// shouldMerge if the intervals overlap or form a continuous Range.
func shouldMerge(a, b Range) bool {
	return a.start <= b.end && b.start <= a.end
}

// mergeRanges creates a new Range with the min start and max end.
func mergeRanges(a, b Range) Range {
	return Range{
		start: min(a.start, b.start),
		end:   max(a.end, b.end),
	}
}

// lessRange compares two ranges for sorting.
func lessRange(a, b Range) bool {
	return a.start < b.start || (a.start == b.start && a.end < b.end)
}

// convertInt64 converts a uint64 to int64. Returns 0 if the value exceeds math.MaxInt64.
func convertInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return 0
	}
	return int64(v)
}

// InvertRanges returns all the gaps between the ranges in sorted order. Assumes that the passed in RangeList is sorted.
func InvertRanges(sorted RangeList) RangeList {
	inverted := make([]Range, 0, len(sorted)+1)
	var prevEnd uint64
	for _, r := range sorted {
		if r.start > prevEnd {
			inverted = append(inverted, Range{start: prevEnd, end: r.start})
		}
		if r.end > prevEnd {
			prevEnd = r.end
		}
	}
	if prevEnd != unboundedEnd {
		inverted = append(inverted, Range{start: prevEnd, end: unboundedEnd})
	}
	return inverted
}
