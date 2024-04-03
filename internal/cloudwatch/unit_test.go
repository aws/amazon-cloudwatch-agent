// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleUnit(t *testing.T) {
	// Each element in the slice has the input and expected output.
	testCases := [][2]string{
		{"", "None"},
		{"1", "None"},
		{"B", "Bytes"},
		{"By", "Bytes"},
		{"by", "Bytes"},
		{"B/s", "Bytes/Second"},
		{"BY/S", "Bytes/Second"},
		{"Bi/s", "Bits/Second"},
		{"Bi", "Bits"},
		{"None", "None"},
		{"Percent", "Percent"},
		{"%", "Percent"},
	}

	for _, testCase := range testCases {
		unit, scale, err := ToStandardUnit(testCase[0])
		assert.NoError(t, err)
		assert.Equal(t, testCase[1], unit)
		assert.EqualValues(t, 1, scale)
	}
}

// If the unit cannot be converted then use None and return an error.
func TestUnsupportedUnit(t *testing.T) {
	testCases := []string{"banana", "ks"}
	for _, testCase := range testCases {
		got, scale, err := ToStandardUnit(testCase)
		assert.Error(t, err)
		assert.Equal(t, "None", got)
		assert.EqualValues(t, 1, scale)
	}
}

func TestScaledUnits(t *testing.T) {
	testCases := []struct {
		input string
		unit  string
		scale float64
	}{
		{"MiBy", "Megabytes", 1.048576},
		{"mby", "Megabytes", 1},
		{"kB", "Kilobytes", 1},
		{"kib/s", "Kilobytes/Second", 1.024},
		{"ms", "Milliseconds", 1},
		{"ns", "Microseconds", 0.001},
		{"min", "Seconds", 60},
		{"h", "Seconds", 60 * 60},
		{"d", "Seconds", 24 * 60 * 60},
	}
	for _, testCase := range testCases {
		unit, scale, err := ToStandardUnit(testCase.input)
		assert.NoError(t, err)
		assert.Equal(t, testCase.unit, unit)
		assert.Equal(t, testCase.scale, scale)
	}
}
