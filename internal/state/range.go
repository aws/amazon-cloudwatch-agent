// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"bytes"
	"encoding"
	"errors"
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
	minCapacity        = 2
)

// Range represents an interval [start, end).
type Range struct {
	start, end uint64
	// seq handles file truncation, when file is truncated, we increase the seq
	seq uint64
}

var _ encoding.TextMarshaler = (*Range)(nil)
var _ encoding.TextUnmarshaler = (*Range)(nil)

func (r *Range) Set(start, end uint64) {
	if start < r.start {
		r.seq++
	}
	r.start = start
	r.end = end
}

func (r *Range) SetInt64(start, end int64) {
	if start < 0 || end < 0 {
		return
	}
	r.Set(uint64(start), uint64(end))
}

func (r Range) Get() (uint64, uint64) {
	return r.start, r.end
}

func (r Range) GetInt64() (int64, int64) {
	var start, end int64
	if r.start <= math.MaxInt64 {
		start = int64(r.start)
	}
	if r.end <= math.MaxInt64 {
		end = int64(r.end)
	}
	return start, end
}

func (r Range) IsEndUnbounded() bool {
	return r.end == unboundedEnd
}

func (r Range) IsValid() bool {
	return r.start < r.end
}

func (r Range) Contains(other Range) bool {
	return r.start <= other.start && r.end >= other.end
}

func (r Range) String() string {
	if r.IsEndUnbounded() {
		return fmt.Sprintf("%v-", r.start)
	}
	return fmt.Sprintf("%d-%d", r.start, r.end)
}

func (r Range) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

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

type RangeTree struct {
	cap  int
	tree *btree.BTreeG[Range]
}

var _ encoding.TextMarshaler = (*RangeTree)(nil)
var _ encoding.TextUnmarshaler = (*RangeTree)(nil)

func NewRangeTree() *RangeTree {
	return NewRangeTreeWithCap(-1)
}

func NewRangeTreeWithCap(capacity int) *RangeTree {
	return &RangeTree{
		cap:  capacity,
		tree: btree.NewG(defaultBTreeDegree, lessRange),
	}
}

func (t *RangeTree) Insert(r Range) bool {
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
	if t.cap >= minCapacity && t.tree.Len() > t.cap {
		t.collapseOldest()
	}
	return true
}

func (t *RangeTree) collapseOldest() bool {
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
		return false
	}
	t.tree.Delete(*first)
	t.tree.Delete(*second)

	merged := mergeRanges(*first, *second)
	t.tree.ReplaceOrInsert(merged)
	return true
}

func (t *RangeTree) Ranges() []Range {
	ranges := make([]Range, 0, t.tree.Len())
	t.tree.Ascend(func(item Range) bool {
		ranges = append(ranges, item)
		return true
	})
	return ranges
}

func (t *RangeTree) len() int {
	return t.tree.Len()
}

func (t *RangeTree) clear() {
	t.tree.Clear(false)
}

func (t *RangeTree) String() string {
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

func (t *RangeTree) MarshalText() ([]byte, error) {
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

func (t *RangeTree) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}
	lines := bytes.Split(text, []byte("\n"))
	if len(lines) < 2 {
		return errors.New("invalid format: missing newline separator")
	}
	maxOffset, err := strconv.ParseUint(string(lines[0]), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid max offset: %w", err)
	}
	defer func() {
		if t.tree.Len() == 0 {
			t.Insert(Range{start: 0, end: maxOffset})
		}
	}()
	parts := bytes.Split(lines[1], []byte(","))
	for _, part := range parts {
		var r Range
		if err = r.UnmarshalText(part); err != nil {
			return fmt.Errorf("invalid range: %w", err)
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

func lessRange(a, b Range) bool {
	return a.start < b.start || (a.start == b.start && a.end < b.end)
}

func Invert(sorted []Range) []Range {
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
