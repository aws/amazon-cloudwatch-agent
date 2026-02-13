// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// FileProvider reads an OIDC token from a file on disk. The user is
// responsible for keeping the file current (e.g., via a sidecar or cron job).
type FileProvider struct {
	path string
}

func NewFileProvider(path string) *FileProvider {
	return &FileProvider{path: path}
}

func (f *FileProvider) GetToken(_ context.Context) (string, time.Duration, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return "", 0, fmt.Errorf("read token file %s: %w", f.path, err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", 0, fmt.Errorf("token file %s is empty", f.path)
	}
	// No expiry info available from a file; return zero so the caller
	// relies on STS credential expiry for refresh scheduling.
	return token, 0, nil
}

func (f *FileProvider) IsAvailable(_ context.Context) bool {
	_, err := os.Stat(f.path)
	return err == nil
}

func (f *FileProvider) Name() string { return "file" }
