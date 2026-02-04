// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import "strings"

// MaskValue masks sensitive values for logging.
// Shows first 4 characters followed by "..." for values longer than 4 chars.
// Returns "<empty>" for empty strings, "<present>" for short values.
func MaskValue(value string) string {
	if value == "" {
		return "<empty>"
	}
	if len(value) <= 4 {
		return "<present>"
	}
	return value[:4] + "..."
}

// MaskIPAddress masks IP addresses for logging.
// For IPv4, shows first two octets (e.g., "10.0.x.x").
// Returns "<empty>" for empty strings, "<present>" for non-IPv4 formats.
func MaskIPAddress(ip string) string {
	if ip == "" {
		return "<empty>"
	}
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".x.x"
	}
	return "<present>"
}
