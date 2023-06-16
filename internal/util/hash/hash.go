// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"fmt"
	"hash/fnv"
)

// HashName creates a new 32 bit FNV-1a, non-cryptographic
func HashName(value string) string {
	if value == "" {
		return ""
	}

	h := fnv.New32a()
	h.Write([]byte(value))
	return fmt.Sprint(h.Sum32())
}
