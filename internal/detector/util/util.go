// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"path/filepath"
	"strings"
)

func BaseExe(exe string) string {
	base := strings.Trim(exe, "\"")
	if base != "" {
		base = filepath.Base(base)
	}
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return strings.ToLower(base)
}

func TrimQuotes(s string) string {
	return strings.Trim(s, `"'`)
}

func IsValidPort(port int) bool {
	return port >= 0 && port <= 65535
}
