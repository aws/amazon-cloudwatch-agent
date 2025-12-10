// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
)

const (
	suffixDeleted = " (deleted)"
)

// BaseName returns the base filename from a path. Removes quotes and deleted file indicators. Preserves file
// extensions and casing.
func BaseName(path string) string {
	trimmed := strings.Trim(path, "\"")
	if trimmed == "" {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(trimmed), suffixDeleted)
}

// BaseExe returns the executable name from a path. Normalizes the name by removing file extensions and converting to
// lowercase to support cross-platform comparisons.
func BaseExe(exe string) string {
	base := BaseName(exe)
	if base == "" {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
}

func AbsPath(ctx context.Context, process detector.Process, path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	cwd, err := process.CwdWithContext(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, path), nil
}

func TrimQuotes(s string) string {
	return strings.Trim(s, `"'`)
}

func IsValidPort(port int) bool {
	return port >= 0 && port <= 65535
}
