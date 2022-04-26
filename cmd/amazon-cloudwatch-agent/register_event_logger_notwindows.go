// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package main

func RegisterEventLogger() error {
	// Unreachable code, do nothing
	return nil
}