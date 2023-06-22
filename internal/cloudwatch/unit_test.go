// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleUnit(t *testing.T) {
	// Each element in the slice has the input and expectedOutput.
	cases := [][2]string{
		{"", "None"},
		{"1", "None"},
		{"B", "Bytes"},
		{"B/s", "Bytes/Second"},
		{"By/s", "Bytes/Second"},
		{"Bi/s", "Bits/Second"},
		{"Bi", "Bits"},
		{"None", "None"},
		{"Percent", "Percent"},
		{"%", "Percent"},
	}

	for _, c := range cases {
		a, s, err := ToStandardUnit(c[0])
		assert.NoError(t, err)
		assert.Equal(t, c[1], a)
		assert.EqualValues(t, 1, s)
	}
}

// If the unit cannot be converted then use None.
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
		input   string
		unit    string
		scale   float64
		epsilon float64
	}{
		{"MiBy", "Megabytes", 1.049, 0.001},
		{"kB", "Kilobytes", 1, 0},
		{"min", "Seconds", 60, 0},
		{"ns", "Microseconds", 0.001, 0},
	}
	for _, testCase := range testCases {
		unit, scale, err := ToStandardUnit(testCase.input)
		require.NoError(t, err)
		assert.Equal(t, testCase.unit, unit)
		assert.GreaterOrEqual(t, testCase.epsilon, math.Abs(testCase.scale-scale))
	}
}
