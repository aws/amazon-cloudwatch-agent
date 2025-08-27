// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package exph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapToIndexPositiveScale(t *testing.T) {
	tests := []struct {
		name     string
		scale    int
		values   []float64
		expected []int
	}{
		{
			name:     "positive value inside bucket",
			scale:    1,
			values:   []float64{1.3, 1.5, 2.2, 3.9, 4.2, 6.0},
			expected: []int{0, 1, 2, 3, 4, 5},
		},
		{
			// for positive values, histogram buckets use upper-inclusive boundaries
			// this is only reliable on boundaries that are powers of 2
			name:     "positive value is on boundary",
			scale:    1,
			values:   []float64{2.0, 4.0, 8.0},
			expected: []int{1, 3, 5},
		},
		{
			name:     "negative value inside bucket",
			scale:    1,
			values:   []float64{-1.3, -1.5, -2.2, -3.9, -4.2, -6.0},
			expected: []int{0, 1, 2, 3, 4, 5},
		},
		{
			// for negative values, histogram buckets use lower-inclusive boundaries
			// this is only reliable on boundaries that are powers of 2
			name:     "negative value is on boundary",
			scale:    1,
			values:   []float64{-1.0, -2.0, -4.0},
			expected: []int{0, 2, 4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, value := range tt.values {
				assert.Equal(t, tt.expected[i], MapToIndex(value, tt.scale), "expected value %f to map to index %d with scale %d", value, tt.expected[i], tt.scale)
			}
		})
	}

}

func TestLowerBoundary(t *testing.T) {
	// scale = 1, base = 2^(1/2) or sqrt(2) = 1.41421
	assert.InDelta(t, 1.41421, LowerBoundary(1, 1), 0.01) // 2^(1/2)
	assert.InDelta(t, 2.0, LowerBoundary(2, 1), 0.01)     // 2^(2/2)
	assert.InDelta(t, 2.82842, LowerBoundary(3, 1), 0.01) // 2^(3/2)
	assert.InDelta(t, 4.0, LowerBoundary(4, 1), 0.01)     // 2^(4/2)
	assert.InDelta(t, 8.0, LowerBoundary(6, 1), 0.01)     // 2^(6/2)
	assert.InDelta(t, 16.0, LowerBoundary(8, 1), 0.01)    // 2^(8/2)

	// scale = 2, base = 2^(1/4) = 1.18921
	assert.InDelta(t, 1.18921, LowerBoundary(1, 2), 0.01) // 2^(1/4)
	assert.InDelta(t, 1.41421, LowerBoundary(2, 2), 0.01) // 2^(2/4)
	assert.InDelta(t, 1.68180, LowerBoundary(3, 2), 0.01) // 2^(3/4)
	assert.InDelta(t, 2.0, LowerBoundary(4, 2), 0.01)     // 2^(4/4)
	assert.InDelta(t, 2.82842, LowerBoundary(6, 2), 0.01) // 2^(6/8)
	assert.InDelta(t, 4.0, LowerBoundary(8, 2), 0.01)     // 2^(8/8)

	// scale = 0, base = 2
	assert.Equal(t, 1.0, LowerBoundary(0, 0)) // 2^0
	assert.Equal(t, 2.0, LowerBoundary(1, 0)) // 2^1
	assert.Equal(t, 4.0, LowerBoundary(2, 0)) // 2^2
	assert.Equal(t, 8.0, LowerBoundary(3, 0)) // 2^3

	assert.Equal(t, 1.0, LowerBoundary(0, -1))  // 4^0
	assert.Equal(t, 4.0, LowerBoundary(1, -1))  // 4^1
	assert.Equal(t, 16.0, LowerBoundary(2, -1)) // 4^2
	assert.Equal(t, 64.0, LowerBoundary(3, -1)) // 4^3

	// scale = -2, base = 2^(2^2) = 2^4 = 16
	assert.Equal(t, 1.0, LowerBoundary(0, -2))    // 16^0
	assert.Equal(t, 16.0, LowerBoundary(1, -2))   // 16^1
	assert.Equal(t, 256.0, LowerBoundary(2, -2))  // 16^2
	assert.Equal(t, 4096.0, LowerBoundary(3, -2)) // 16^3

	assert.Equal(t, 1.0, LowerBoundary(0, -1))  // 4^0
	assert.Equal(t, 4.0, LowerBoundary(1, -1))  // 2^(2^1)^1 = 4^1 = 2^2 = 4^1
	assert.Equal(t, 16.0, LowerBoundary(2, -1)) // (2^2^1)^2 = 4^2 = 2^4 = 4^2
	assert.Equal(t, 64.0, LowerBoundary(3, -1)) // (2^2^1)^3 = 4^3 = 2^6 = 4^3

	// scale = -2, base = 2^(2^2) = 2^4 = 16
	assert.Equal(t, 1.0, LowerBoundary(0, -2))    // (2^(2^2))^0 = 16^0 = 1
	assert.Equal(t, 16.0, LowerBoundary(1, -2))   // (2^(2^2))^1 = 2^4 = 16^1
	assert.Equal(t, 256.0, LowerBoundary(2, -2))  // (2^(2^2))^2 = 2^8 = 16^2
	assert.Equal(t, 4096.0, LowerBoundary(3, -2)) // (2^(2^2))^3 = 2^12 = 16^3
}

func BenchmarkLowerBoundary(b *testing.B) {
	b.Run("positive scale", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			LowerBoundary(10, 1)
		}
	})

	b.Run("scale 0", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			LowerBoundary(10, 0)
		}
	})

	b.Run("negative scale", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			LowerBoundary(10, -1)
		}
	})
}

func BenchmarkLowerBoundaryNegativeScale(b *testing.B) {
	b.Run("reference", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			LowerBoundaryNegativeScale(10, -9)
		}
	})
}
