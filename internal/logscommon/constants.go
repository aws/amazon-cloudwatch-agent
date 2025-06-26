// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logscommon

const (
	// CloudWatch Logs API metadata overhead per event
	CwLogsHeaderBytes = 200

	// Maximum nominal event size (256KiB)
	MaxNominalEventSize = 262144 // 256 * 1024 bytes

	// Maximum effective event size (256KiB - header overhead)
	MaxEffectiveEventSize = MaxNominalEventSize - CwLogsHeaderBytes // 261,944 bytes

	// Buffer size for reading (no extra padding needed)
	ReadBufferSize = MaxEffectiveEventSize

	// Default truncate suffix
	DefaultTruncateSuffix = "[Truncated...]"
)

// ValidateEventSize ensures an event doesn't exceed the maximum size
func ValidateEventSize(event string) string {
	if len(event) > MaxEffectiveEventSize {
		truncatePoint := MaxEffectiveEventSize - len(DefaultTruncateSuffix)
		if truncatePoint < 0 {
			truncatePoint = 0
		}
		return event[:truncatePoint] + DefaultTruncateSuffix
	}
	return event
}

// CalculateEffectiveSize returns the effective size for a given nominal size
func CalculateEffectiveSize(nominalSize int) int {
	return nominalSize - CwLogsHeaderBytes
}
