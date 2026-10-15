// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aws/amazon-cloudwatch-agent/internal/constants"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

const (
	otelConfigFlagName = "-otelconfig"
)

// GetOTELConfigArgs creates otelconfig argument pairs for YAML files in dir.
// The translated agent YAML is appended last only when that file exists;
// config-translator omits it when there are no OTEL pipelines.
func GetOTELConfigArgs(dir string) []string {
	configs := getSortedYAMLs(dir)
	if _, err := os.Stat(paths.YamlConfigPath); err == nil {
		configs = append(configs, paths.YamlConfigPath)
	}
	args := make([]string, 0, 2*len(configs))
	for _, config := range configs {
		args = append(args, otelConfigFlagName, config)
	}
	return args
}

// getSortedYAMLs gets an ordered slice of all the YAML files in the directory. Uses filepath.WalkDir which walks the
// files in lexical order making the result deterministic.
func getSortedYAMLs(dir string) []string {
	var configs []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if d == nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == constants.FileSuffixYAML {
			configs = append(configs, path)
		}
		return nil
	})
	return configs
}
