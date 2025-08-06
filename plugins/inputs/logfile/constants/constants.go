// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package constants

const (
	// DefaultReaderBufferSize is the default buffer size for file readers (256KB)
	// This is much smaller than MaxEventSize to reduce memory usage
	DefaultReaderBufferSize = 256 * 1024

	// DefaultMaxEventSize is the default maximum size for log events (1MB)
	DefaultMaxEventSize = 1024 * 1024

	// PerEventHeaderBytes is the bytes required for metadata for each log event
	PerEventHeaderBytes = 200
)
