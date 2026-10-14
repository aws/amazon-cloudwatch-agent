// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

// setupPaths redirects all relevant package-level path vars into a temp
// directory and returns a cleanup function that restores the originals.
// TranslatorBinaryPath is set to a guaranteed-absent path so that any call
// to translateConfig() that reaches the subprocess step fails fast rather
// than accidentally invoking a binary on the host machine.
func setupPaths(t *testing.T) (dir string, cleanup func()) {
	t.Helper()
	dir = t.TempDir()

	origToml := paths.TomlConfigPath
	origJson := paths.JsonConfigPath
	origCfgDir := paths.ConfigDirPath
	origCommon := paths.CommonConfigPath
	origEnvCfg := paths.EnvConfigPath
	origAgentBin := paths.AgentBinaryPath
	origTranslator := paths.TranslatorBinaryPath

	paths.TomlConfigPath = filepath.Join(dir, "agent.toml")
	paths.JsonConfigPath = filepath.Join(dir, "agent.json")
	paths.ConfigDirPath = filepath.Join(dir, "Configs")
	paths.CommonConfigPath = filepath.Join(dir, "common-config.toml")
	paths.EnvConfigPath = filepath.Join(dir, "env-config.json")
	paths.AgentBinaryPath = filepath.Join(dir, "amazon-cloudwatch-agent.exe")
	paths.TranslatorBinaryPath = filepath.Join(dir, "config-translator-does-not-exist.exe")

	if err := os.MkdirAll(paths.ConfigDirPath, 0755); err != nil {
		t.Fatalf("mkdir Configs: %v", err)
	}

	return dir, func() {
		paths.TomlConfigPath = origToml
		paths.JsonConfigPath = origJson
		paths.ConfigDirPath = origCfgDir
		paths.CommonConfigPath = origCommon
		paths.EnvConfigPath = origEnvCfg
		paths.AgentBinaryPath = origAgentBin
		paths.TranslatorBinaryPath = origTranslator
	}
}

// writeFile creates a file at path with content "x" and the given mtime.
func writeFile(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

// freshOutputs writes both required outputs (TOML and env-config.json) with
// the given mtime. Most tests that expect stale=false call this helper.
func freshOutputs(t *testing.T, mtime time.Time) {
	t.Helper()
	writeFile(t, paths.TomlConfigPath, mtime)
	writeFile(t, paths.EnvConfigPath, mtime)
}

// --- isTranslationStale tests ---

func TestIsTranslationStale_NoToml(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	// Neither TOML nor env-config exists.
	if !isTranslationStale() {
		t.Error("expected stale=true when TOML does not exist")
	}
}

func TestIsTranslationStale_NoEnvConfig(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	// TOML is fresh but env-config.json is missing (incomplete prior translation).
	base := time.Now().Truncate(time.Second)
	writeFile(t, paths.TomlConfigPath, base.Add(time.Second))
	writeFile(t, paths.JsonConfigPath, base)
	writeFile(t, paths.CommonConfigPath, base)
	// EnvConfigPath intentionally absent.

	if !isTranslationStale() {
		t.Error("expected stale=true when env-config.json is missing")
	}
}

func TestIsTranslationStale_Fresh(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	base := time.Now().Truncate(time.Second)

	// Outputs are newest; no translation needed.
	writeFile(t, paths.JsonConfigPath, base)
	writeFile(t, paths.CommonConfigPath, base)
	freshOutputs(t, base.Add(time.Second))

	if isTranslationStale() {
		t.Error("expected stale=false when both outputs are newer than all inputs")
	}
}

func TestIsTranslationStale_JsonNewer(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	base := time.Now().Truncate(time.Second)

	freshOutputs(t, base)
	writeFile(t, paths.JsonConfigPath, base.Add(time.Second)) // newer than outputs
	writeFile(t, paths.CommonConfigPath, base)

	if !isTranslationStale() {
		t.Error("expected stale=true when JsonConfigPath is newer than outputs")
	}
}

func TestIsTranslationStale_CommonConfigNewer(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	base := time.Now().Truncate(time.Second)

	freshOutputs(t, base)
	writeFile(t, paths.JsonConfigPath, base)
	writeFile(t, paths.CommonConfigPath, base.Add(time.Second)) // newer

	if !isTranslationStale() {
		t.Error("expected stale=true when CommonConfigPath is newer than outputs")
	}
}

func TestIsTranslationStale_ConfigDirFileNewer(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	base := time.Now().Truncate(time.Second)

	freshOutputs(t, base)
	writeFile(t, paths.JsonConfigPath, base)
	writeFile(t, paths.CommonConfigPath, base)
	// A top-level Configs/ file that is newer than the outputs.
	writeFile(t, filepath.Join(paths.ConfigDirPath, "file_config.json"), base.Add(time.Second))

	if !isTranslationStale() {
		t.Error("expected stale=true when a Configs/ file is newer than outputs")
	}
}

func TestIsTranslationStale_NestedConfigDirFileNewer(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	base := time.Now().Truncate(time.Second)

	freshOutputs(t, base)
	writeFile(t, paths.JsonConfigPath, base)
	writeFile(t, paths.CommonConfigPath, base)

	// A file in a nested Configs/sub/ directory that is newer than the outputs.
	// The translator walks ConfigDirPath recursively; isTranslationStale must too.
	subdir := filepath.Join(paths.ConfigDirPath, "sub")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	writeFile(t, filepath.Join(subdir, "nested.json"), base.Add(time.Second))

	if !isTranslationStale() {
		t.Error("expected stale=true when a nested Configs/sub/ file is newer than outputs")
	}
}

func TestIsTranslationStale_AgentBinaryNewer(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	base := time.Now().Truncate(time.Second)

	freshOutputs(t, base)
	writeFile(t, paths.JsonConfigPath, base)
	writeFile(t, paths.CommonConfigPath, base)
	// Agent binary is newer than outputs — simulates an in-place binary upgrade.
	// The updated binary may produce different TOML output, so retranslation is required.
	writeFile(t, paths.AgentBinaryPath, base.Add(time.Second))

	if !isTranslationStale() {
		t.Error("expected stale=true when agent binary is newer than outputs")
	}
}

func TestIsTranslationStale_AgentBinaryAbsent(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	base := time.Now().Truncate(time.Second)

	// Binary does not exist in the temp dir (simulates a path misconfiguration
	// or a very first install where the binary was checked before creation).
	// A missing binary is skipped and must not force retranslation.
	freshOutputs(t, base.Add(time.Second))
	writeFile(t, paths.JsonConfigPath, base)
	writeFile(t, paths.CommonConfigPath, base)
	// AgentBinaryPath intentionally absent.

	if isTranslationStale() {
		t.Error("expected stale=false when agent binary path does not exist")
	}
}

func TestIsTranslationStale_NoInputsExist(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	// Both outputs exist; no input files exist. Nothing to compare → fresh.
	freshOutputs(t, time.Now())

	if isTranslationStale() {
		t.Error("expected stale=false when outputs exist and no input files exist")
	}
}

func TestIsTranslationStale_SameTimestamp(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	base := time.Now().Truncate(time.Second)

	// Inputs and outputs share the exact same mtime. .After() is strict, so
	// same-second timestamps are treated as fresh.
	writeFile(t, paths.JsonConfigPath, base)
	writeFile(t, paths.CommonConfigPath, base)
	freshOutputs(t, base)

	if isTranslationStale() {
		t.Error("expected stale=false when outputs and inputs share the same mtime")
	}
}

func TestIsTranslationStale_EnvConfigOlderThanToml(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	base := time.Now().Truncate(time.Second)

	// TOML is fresh relative to inputs but env-config.json is older.
	// The threshold is the older of the two outputs, so an input at base+1 s
	// should still trigger re-translation.
	writeFile(t, paths.TomlConfigPath, base.Add(2*time.Second))
	writeFile(t, paths.EnvConfigPath, base)                   // older output → sets threshold
	writeFile(t, paths.JsonConfigPath, base.Add(time.Second)) // newer than env-config

	if !isTranslationStale() {
		t.Error("expected stale=true when an input is newer than the older output (env-config.json)")
	}
}

// --- translateConfig container-mode guard test ---

// TestTranslateConfig_ContainerMode_AlwaysTranslates verifies that when
// RUN_IN_CONTAINER=true the early-return guard is bypassed and the subprocess
// is attempted even when the TOML appears fresh. The subprocess will fail
// (TranslatorBinaryPath is absent in tests), but the important assertion is
// that translateConfig returns a non-nil error rather than nil (which would
// indicate the skip path was taken).
func TestTranslateConfig_ContainerMode_AlwaysTranslates(t *testing.T) {
	_, cleanup := setupPaths(t)
	defer cleanup()

	t.Setenv("RUN_IN_CONTAINER", "True") // envconfig.TrueValue = "True"

	// Set up outputs that look completely fresh.
	base := time.Now().Truncate(time.Second)
	freshOutputs(t, base.Add(time.Second))
	writeFile(t, paths.JsonConfigPath, base)
	writeFile(t, paths.CommonConfigPath, base)

	// In container mode the guard must not return nil early; it must attempt
	// to run the translator (which doesn't exist here, so it returns an error).
	err := translateConfig()
	if err == nil {
		t.Error("expected non-nil error: container mode must always attempt translation, never skip it")
	}
}
