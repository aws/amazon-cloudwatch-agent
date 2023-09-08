// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToOtelValue(t *testing.T) {
	testCases := []struct {
		input interface{}
		want  interface{}
	}{
		{input: math.NaN(), want: nil},
		{input: math.Inf(1), want: nil},
		{input: math.Inf(-1), want: nil},
		{input: "test", want: nil},
		{input: int32(3), want: int64(3)},
		{input: 5.5, want: 5.5},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.want, ToOtelValue(testCase.input))
	}
}
