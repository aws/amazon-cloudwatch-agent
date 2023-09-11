// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package distribution

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAcceptedValue(t *testing.T) {
	testCases := []struct {
		input float64
		want  bool
	}{
		{input: MinValue * 1.0001, want: false},
		{input: MinValue, want: true},
		{input: MaxValue, want: true},
		{input: MaxValue * 1.0001, want: false},
		{input: math.Pow(2, 300), want: true},
		{input: math.NaN(), want: false},
		{input: math.Inf(1), want: false},
		{input: math.Inf(-1), want: false},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.want, IsSupportedValue(testCase.input, MinValue, MaxValue))
	}
}
