// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package envconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
)

// ReadFile parses the env-config.json at the given path and returns the
// key-value pairs. Returns an error if the file is missing or cannot be parsed.
func ReadFile(path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	envVars := map[string]string{}
	if err = json.Unmarshal(data, &envVars); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return envVars, nil
}

// LoadFile loads environment variables from the given env-config.json into
// the process environment, logging each variable set and any that fail to set.
func LoadFile(path string) error {
	envVars, err := ReadFile(path)
	if err != nil {
		return err
	}
	for k, v := range envVars {
		if err = os.Setenv(k, v); err != nil {
			log.Printf("W! Failed to set environment variable %s: %v", k, err)
			continue
		}
		log.Printf("I! %s is set to %q", k, v)
	}
	return nil
}

// MergeFile merges values into the env-config.json at path. Existing keys are
// retained unless listed in keysToRemove. A missing file is treated as empty.
// Other read errors leave the file untouched.
func MergeFile(path string, values map[string]string, keysToRemove ...string) error {
	if path == "" {
		return nil
	}
	merged, err := ReadFile(path)
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
	data, err := json.MarshalIndent(merged, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644) //nolint:gosec // G306: 0644 is intentional for env-config.json
}
