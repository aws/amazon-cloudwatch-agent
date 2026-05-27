// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package envconfig

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
)

// ReadEnvConfigFile parses the env-config.json at the given path and returns the key-value pairs.
func ReadEnvConfigFile(path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	envVars := map[string]string{}
	if err = json.Unmarshal(data, &envVars); err != nil {
		return nil, err
	}
	return envVars, nil
}

// LoadEnvConfigFile loads environment variables from the given env-config.json into the process environment.
func LoadEnvConfigFile(path string) error {
	envVars, err := ReadEnvConfigFile(path)
	if err != nil {
		return err
	}
	for k, v := range envVars {
		os.Setenv(k, v)
	}
	return nil
}

// MergeEnvConfigFile merges the given values into the env-config.json at path.
// Existing values are retained; the provided values take precedence.
func MergeEnvConfigFile(path string, values map[string]string) error {
	return ReplaceEnvConfigFile(path, values, nil)
}

// ReplaceEnvConfigFile merges the given values into the env-config.json at path,
// first removing keysToRemove so stale values don't persist.
func ReplaceEnvConfigFile(path string, values map[string]string, keysToRemove []string) error {
	if path == "" {
		return nil
	}
	merged, err := ReadEnvConfigFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if merged == nil {
		merged = make(map[string]string)
	}
	for _, k := range keysToRemove {
		delete(merged, k)
	}
	for k, v := range values {
		merged[k] = v
	}
	bytes, err := json.MarshalIndent(merged, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0644) //nolint:gosec // retains existing permissions
}
