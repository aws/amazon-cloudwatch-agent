// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	_ "embed"
	"regexp"
	"strings"
)

// SchemaURL is the published JSON Schema for the agent config (#1219).
// The installed copy is amazon-cloudwatch-agent-schema.json; this URL is the same document.
const SchemaURL = "https://github.com/aws/amazon-cloudwatch-agent/raw/main/translator/config/schema.json"

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
