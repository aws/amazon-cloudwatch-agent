// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/regular"
)

func TestToOtelValue(t *testing.T) {
	distribution := regular.NewRegularDistribution()
	testCases := []struct {
		input interface{}
		want  interface{}
	}{
		// ints
		{input: 5, want: int64(5)},
		{input: int8(5), want: int64(5)},
		{input: int16(5), want: int64(5)},
		{input: int32(5), want: int64(5)},
		{input: int64(5), want: int64(5)},
		// uints
		{input: uint(5), want: int64(5)},
		{input: uint8(5), want: int64(5)},
		{input: uint16(5), want: int64(5)},
		{input: uint32(5), want: int64(5)},
		{input: uint64(5), want: int64(5)},
		// floats
		{input: float32(5.5), want: 5.5},
		{input: 5.5, want: 5.5},
		// bool
		{input: false, want: int64(0)},
		{input: true, want: int64(1)},
		// distribution
		{input: distribution, want: distribution},
		// unsupported floats
		{input: math.NaN(), want: nil},
		{input: math.Inf(1), want: nil},
		{input: math.Inf(-1), want: nil},
		// unsupported types
		{input: "test", want: nil},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.want, ToOtelValue(testCase.input))
	}
}
