// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertUnit(t *testing.T) {
	// Each element in the slice has the input and expectedOutput.
	cases := [][2]string{
		{"", "None"},
		{"1", "None"},
		{"B", "Bytes"},
		{"B/s", "Bytes/Second"},
		{"By/s", "Bytes/Second"},
		{"Bi/s", "Bits/Second"},
		{"banana", "banana"},
		{"KBi", "Kilobits"},
	}

	for _, c := range cases {
		a := ConvertUnit(c[0])
		assert.Equal(t, c[1], a)
	}
}

// If the unit cannot be converted then just use the current value.
func TestConvertUnitNoMatch(t *testing.T) {
	assert.Equal(t, "Bytes", ConvertUnit("Bytes"))
}
