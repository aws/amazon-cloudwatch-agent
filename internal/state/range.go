// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type FileRange struct {
	Start, End uint64
}

func (r FileRange) IsValid() bool {
	return r.Start < r.End
}

type FileRanges struct {
	ranges []FileRange
}

func (r *FileRanges) Add(v FileRange) {
	if !v.IsValid() {
		return
	}
	index := sort.Search(len(r.ranges), func(i int) bool {
		return r.ranges[i].Start > v.Start
	})

	if index > 0 && r.ranges[index-1].End+1 >= v.Start {
		if v.End > r.ranges[index-1].End {
			r.ranges[index-1].End = v.End
		}
		r.mergeFrom(index - 1)
		return
	}

	r.ranges = append(r.ranges, FileRange{})
	copy(r.ranges[index+1:], r.ranges[index:])
	r.ranges[index] = v
	r.mergeFrom(index)
}

func (r *FileRanges) mergeFrom(index int) {
	if index >= len(r.ranges) {
		return
	}
	current := &r.ranges[index]
	mergeEnd := index

	for i := index + 1; i < len(r.ranges); i++ {
		if current.End+1 >= r.ranges[i].Start {
			if r.ranges[i].End > current.End {
				current.End = r.ranges[i].End
			}
			mergeEnd = i
		} else {
			break
		}
	}

	if mergeEnd > index {
		r.ranges = append(r.ranges[:index+1], r.ranges[mergeEnd+1:]...)
	}
}

func (r FileRanges) MarshalText() ([]byte, error) {
	var buf bytes.Buffer
	for index, v := range r.ranges {
		if index > 0 {
			buf.WriteByte(',')
		}
		_, err := fmt.Fprintf(&buf, "%d-%d", v.Start, v.End)
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (r *FileRanges) UnmarshalText(b []byte) error {
	r.ranges = make([]FileRange, 0)
	if len(b) == 0 {
		return nil
	}
	str := strings.Split(string(b), ",")
	for _, v := range str {
		parts := strings.Split(v, "-")
		if len(parts) != 2 {
			return fmt.Errorf("invalid range: %s", v)
		}
		start, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return err
		}
		end, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return err
		}
		r.Add(FileRange{start, end})
	}
	return nil
}
