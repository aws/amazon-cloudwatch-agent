// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package main

// requestSCMStop and handleTerminatingSignal are no-ops on non-Windows.
func requestSCMStop() bool     { return false }
func handleTerminatingSignal() {}
