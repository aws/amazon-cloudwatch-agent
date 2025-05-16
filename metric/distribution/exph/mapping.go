// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// The following functions are originally sourced from OpenTelemetry's reference implementation. See
// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#producer-expectations
package exph

import "math"

// MapToIndexScale0 computes a bucket index at scale 0.
func MapToIndexScale0(value float64) int {
	// Note: Frexp() rounds submnormal values to the smallest normal
	// value and returns an exponent corresponding to fractions in the
	// range [0.5, 1), whereas an exponent for the range [1, 2), so
	// subtract 1 from the exponent immediately.
	frac, exp := math.Frexp(value)
	exp--

	if frac == 0.5 && value > 0 {
		// Special case for positive powers of two: they fall into the bucket
		// numbered one less.
		exp--
	}
	return exp
}

// MapToIndexNegativeScale computes a bucket index for scales <= 0.
func MapToIndexNegativeScale(value float64, scale int) int {
	return MapToIndexScale0(value) >> -scale
}

// MapToIndex for any scale
//
// Values near a boundary could be mapped into the incorrect bucket due to float point calculation inaccuracy.
func MapToIndex(value float64, scale int) int {
	// Special case for power-of-two values.
	if frac, exp := math.Frexp(value); frac == 0.5 {
		return ((exp - 1) << scale) - 1
	}
	scaleFactor := math.Ldexp(math.Log2E, scale)
	// The use of math.Log() to calculate the bucket index is not guaranteed to be exactly correct near powers of two.
	return int(math.Floor(math.Log(math.Abs(value)) * scaleFactor))
}

// LowerBoundaryNegativeScale computes the lower boundary for index
// with scales <= 0.
func LowerBoundaryNegativeScale(index int, scale int) float64 {
	return math.Ldexp(1, index<<-scale)
}

func LowerBoundary(index, scale int) float64 {
	if scale <= 0 {
		return LowerBoundaryNegativeScale(index, scale)
	}
	return LowerBoundaryPositiveScale(index, scale)
}

// LowerBoundary computes the bucket boundary for positive scales.
//
// The returned value may be inaccurate due to accumulated floating point calculation errors
func LowerBoundaryPositiveScale(index, scale int) float64 {
	inverseFactor := math.Ldexp(math.Ln2, -scale)
	return math.Exp(float64(index) * inverseFactor)
}
