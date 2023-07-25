// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	_ "embed"
	"regexp"
	"strings"
)

//go:embed schema.json
var schema string

func GetJsonSchema() string {
	return schema
}

func OverwriteSchema(newSchema string) {
	schema = newSchema
}

// Translate Sample:
// (root).agent.metrics_collection_interval -> /agent/metrics_collection_interval
// (root).metrics.metrics_collected.cpu.resources.1 -> /metrics/metrics_collected/cpu/resources/1
func GetFormattedPath(rawPath string) string {
	//replace heading (root). to /
	prefixRe := regexp.MustCompile("^\\(root\\).")
	result := prefixRe.ReplaceAllString(rawPath, "/")
	//replace . to /
	result = strings.Replace(result, ".", "/", -1)
	return result
}
