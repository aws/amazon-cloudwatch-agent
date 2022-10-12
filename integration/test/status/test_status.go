// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package status

type TestStatus string

const (
	SUCCESSFUL TestStatus = "Successful"
	FAILED     TestStatus = "Failed"
)
