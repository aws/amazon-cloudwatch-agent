// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// HashName creates a new 32 bit FNV-1a, non-cryptographic
func TestHashFNV(t *testing.T) {
	const (
		text       = "hello, world!"
		FNV1Digest = "2316077684"
	)

	actualHash := HashName(text)
	require.Equal(t, FNV1Digest, actualHash)

	// Empty hash
	require.Equal(t, HashName(""), "")
}

func BenchmarkHashFNV(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashName(uuid.NewString())
	}
}
