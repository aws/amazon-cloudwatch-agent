// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package name

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExcludeFilter(t *testing.T) {
	testCases := map[string]struct {
		input        string
		excludeNames []string
		want         bool
	}{
		"EmptyFilter": {
			input: "anything",
			want:  true,
		},
		"Include": {
			input:        "anything",
			excludeNames: []string{"exclude"},
			want:         true,
		},
		"Exclude": {
			input:        "exclude",
			excludeNames: []string{"exclude"},
			want:         false,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			f := NewExcludeFilter(testCase.excludeNames...)
			assert.Equal(t, testCase.want, f.ShouldInclude(testCase.input))
		})
	}
}
