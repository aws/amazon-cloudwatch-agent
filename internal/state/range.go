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
)

// Range represents an interval [start, end).
type Range struct {
	start, end uint64
}

var _ encoding.TextMarshaler = (*Range)(nil)
var _ encoding.TextUnmarshaler = (*Range)(nil)

func (r Range) IsValid() bool {
	return r.start < r.end
}

func (r Range) Equal(other Range) bool {
	return r.start == other.start && r.end == other.end
}

func (r Range) Contains(other Range) bool {
	return r.start <= other.start && r.end >= other.end
}

func (r Range) String() string {
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
	end, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid end: %s", parts[1])
	}
	*r = Range{start: start, end: end}
	return nil
}

type RangeTree struct {
	tree *btree.BTreeG[Range]
}

var _ encoding.TextMarshaler = (*RangeTree)(nil)
var _ encoding.TextUnmarshaler = (*RangeTree)(nil)

func NewRangeTree() *RangeTree {
	return &RangeTree{tree: btree.NewG(defaultBTreeDegree, lessRange)}
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
	if len(toRemove) == 1 && toRemove.Contains(merged) {
		return false
	}
	for item := range toRemove {
		t.tree.Delete(item)
	}
	t.tree.ReplaceOrInsert(merged)
	return true
}

func (t *RangeTree) Ranges() []Range {
	var ranges []Range
	t.tree.Ascend(func(item Range) bool {
		ranges = append(ranges, item)
		return true
	})
	return ranges
}

func (t *RangeTree) String() string {
	var ranges []string
	t.tree.Ascend(func(item Range) bool {
		ranges = append(ranges, item.String())
		return true
	})
	return "[" + strings.Join(ranges, ",") + "]"
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
	maxEnd, err := strconv.ParseUint(string(lines[0]), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid end: %w", err)
	}
	defer func() {
		if t.tree.Len() == 0 {
			t.Insert(Range{start: 0, end: maxEnd})
		}
	}()
	parts := bytes.Split(lines[1], []byte(","))
	for _, part := range parts {
		var r Range
		if err := r.UnmarshalText(part); err != nil {
			return fmt.Errorf("invalid range %q: %w", part, err)
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

func Gaps(ranges []Range) []Range {
	var gaps []Range
	var prevEnd uint64
	for _, r := range ranges {
		if r.start > prevEnd {
			gaps = append(gaps, Range{start: prevEnd, end: r.start})
		}
		if r.end > prevEnd {
			prevEnd = r.end
		}
	}
	gaps = append(gaps, Range{start: prevEnd, end: math.MaxUint64})
	return gaps
}
