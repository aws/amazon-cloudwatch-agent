// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var testInput = map[string]any{
	"1": map[string]any{
		"1": []any{
			map[string]any{
				"key": "v1",
			},
			map[string]any{
				"key":     "v2",
				"options": []any{"o1", "o2"},
			},
		},
		"2": map[string]any{
			"key": "v3",
		},
	},
	"2": []any{"v4", "v5"},
}

type collectVisitor struct {
	got []any
}

var _ Visitor = (*collectVisitor)(nil)

func (v *collectVisitor) Visit(input any) error {
	v.got = append(v.got, input)
	return nil
}

func TestVisit(t *testing.T) {
	testCases := map[string]struct {
		path    string
		visitor Visitor
		want    []any
		wantErr error
	}{
		"EmptyPath": {
			path:    Path(""),
			wantErr: ErrNoKeysFound,
		},
		"InvalidPath": {
			path:    Path("1", "invalid", "2"),
			wantErr: ErrPathNotFound,
		},
		"InvalidTarget": {
			path:    Path("1", "invalid"),
			wantErr: ErrTargetNotFound,
		},
		"UnsupportedType": {
			path:    Path("1", "2", "key", "invalid"),
			wantErr: ErrUnsupportedType,
		},
		"ValidTarget": {
			path: Path("1", "2"),
			want: []any{
				map[string]any{
					"key": "v3",
				},
			},
		},
		"SliceInPath": {
			path: Path("1", "1", "key"),
			want: []any{"v1", "v2"},
		},
		"PartialTargetMatch": {
			path: Path("1", "1", "options"),
			want: []any{
				[]any{"o1", "o2"},
			},
			wantErr: ErrTargetNotFound,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			v := &collectVisitor{}
			assert.ErrorIs(t, Visit(testInput, testCase.path, v), testCase.wantErr)
			assert.Equal(t, testCase.want, v.got)
		})
	}
}

func TestIsSet(t *testing.T) {
	testCases := map[string]struct {
		path string
		want bool
	}{
		"EmptyPath": {
			path: "",
			want: false,
		},
		"ExistingTarget": {
			path: Path("1", "2", "key"),
			want: true,
		},
		"NonExisting/Target": {
			path: Path("1", "2", "missing"),
			want: false,
		},
		"NonExisting/Path": {
			path: Path("1", "missing", "2"),
			want: false,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.want, IsSet(testInput, testCase.path))
		})
	}
}

func TestSliceVisitor(t *testing.T) {
	cv := &collectVisitor{}
	v := NewSliceVisitor(cv)
	assert.NoError(t, Visit(testInput, Path("2"), v))
	assert.Equal(t, []any{"v4", "v5"}, cv.got)

	assert.ErrorIs(t, Visit(testInput, Path("1"), v), ErrUnsupportedType)

	v = NewSliceVisitor(NewVisitor(func(any) error {
		return assert.AnError
	}))
	assert.ErrorIs(t, Visit(testInput, Path("2"), v), assert.AnError)
}

func TestPath(t *testing.T) {
	testCases := map[string]struct {
		keys []string
		want string
	}{
		"Empty": {
			keys: nil,
			want: "",
		},
		"SingleKey": {
			keys: []string{"key"},
			want: "key",
		},
		"MultipleKeys": {
			keys: []string{"path", "to", "key"},
			want: "path/to/key",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, Path(tc.keys...))
		})
	}
}
