// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
)

// newCapturingLogger returns a logger that captures all output for test assertions.
func newCapturingLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logger, buf
}

func infoLevel() *promslog.AllowedLevel {
	lvl := &promslog.AllowedLevel{}
	_ = lvl.Set("info")
	return lvl
}

// TestCreateTargetAllocatorManager_NoTASection_NoWarn verifies no Warn is logged for a non-TA config.
func TestCreateTargetAllocatorManager_NoTASection_NoWarn(t *testing.T) {
	logger, buf := newCapturingLogger()

	tam := createTargetAllocatorManager("testdata/base-k8.yaml", logger, infoLevel(), nil, nil)

	assert.False(t, tam.enabled, "TA must be disabled")

	out := buf.String()
	assert.NotContains(t, out, "level=WARN", "no Warn expected")
	assert.NotContains(t, out, "Could not load config for target allocator",
		"spurious TA warning must not appear")

	assert.Contains(t, out, "level=DEBUG")
	assert.Contains(t, out, "Target allocator not configured")
}

// TestCreateTargetAllocatorManager_MalformedTASection_Warns verifies a genuine TA config error still warns.
func TestCreateTargetAllocatorManager_MalformedTASection_Warns(t *testing.T) {
	dir := t.TempDir()
	badTA := filepath.Join(dir, "bad-ta.yaml")
	// Has target_allocator section but an invalid key that fails strict unmarshal.
	content := []byte("target_allocator:\n  endpoint: http://target-allocator-service:80\n  interval: 30s\nthis_key_is_invalid: true\n")
	assert.NoError(t, os.WriteFile(badTA, content, 0o600))

	logger, buf := newCapturingLogger()

	tam := createTargetAllocatorManager(badTA, logger, infoLevel(), nil, nil)

	assert.False(t, tam.enabled, "manager stays disabled")
	out := buf.String()
	assert.Contains(t, out, "level=WARN", "genuine TA error must still warn")
	assert.Contains(t, out, "Could not load config for target allocator")
}

// TestLoadConfigFromFilename_HasTA tests the TA section detection in loadConfigFromFilename.
func TestLoadConfigFromFilename_HasTA(t *testing.T) {
	_, hasTA, _ := loadConfigFromFilename("testdata/base-k8.yaml")
	assert.False(t, hasTA, "no TA section")

	_, hasTA, _ = loadConfigFromFilename("testdata/target_allocator.yaml")
	assert.True(t, hasTA, "has TA section")

	_, hasTA, _ = loadConfigFromFilename("testdata/does-not-exist.yaml")
	assert.True(t, hasTA, "missing file defaults to true (warn)")
}
