// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package constants

const (
	// DefaultMaxEventSize is the default maximum size for log events (1MB)
	DefaultMaxEventSize = 1024 * 1024

	// PerEventHeaderBytes is the bytes required for metadata for each log event
	PerEventHeaderBytes = 200

	// DefaultTruncateSuffix is the suffix added to truncated log messages
	DefaultTruncateSuffix = "[Truncated...]"
)
