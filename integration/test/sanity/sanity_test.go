// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build integration
// +build integration

package sanity

import (
	"testing"
)

func TestAgentStatus(t *testing.T) {
	SanityCheck(t)
}
