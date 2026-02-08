// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !linux && !windows

package nvidia

type checker struct{}

// newChecker creates a new NVIDIA GPU checker (stub implementation).
func newChecker() deviceChecker {
	return &checker{}
}

func (c *checker) hasNvidiaDevice() bool {
	return false
}

func (c *checker) hasDriverFiles() bool {
	return false
}
