// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
)

func BaseExe(exe string) string {
	base := strings.Trim(exe, "\"")
	if base != "" {
		base = filepath.Base(base)
	}
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return strings.ToLower(base)
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
