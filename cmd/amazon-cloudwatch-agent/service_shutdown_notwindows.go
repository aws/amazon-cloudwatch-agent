// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package main

// exitAfterServiceRun is a no-op off Windows.
func exitAfterServiceRun() {}
