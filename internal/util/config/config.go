// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"io/fs"
	"path/filepath"

	"github.com/aws/amazon-cloudwatch-agent/internal/constants"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

const (
	otelConfigFlagName = "-otelconfig"
)

// GetOTELConfigArgs creates otelconfig argument pairs for all YAML paths in the directory along with the agent YAML
// path as the last pair.
func GetOTELConfigArgs(dir string) []string {
	configs := getSortedYAMLs(dir)
	configs = append(configs, paths.YamlConfigPath)
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
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == constants.FileSuffixYAML {
			configs = append(configs, path)
		}
		return nil
	})
	return configs
}
