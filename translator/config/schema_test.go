// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetJsonSchema(t *testing.T) {
	jsonFile, err := os.ReadFile("./schema.json")
	if err != nil {
		panic(err)
	}
	str := strings.ReplaceAll(string(jsonFile), "\r\n", "\n")
	assert.Equal(t, str, GetJsonSchema(), "Json schema is inconsistent")
}

func TestGetFormattedPath(t *testing.T) {
	assert.Equal(t, "/metrics/metrics_collected/cpu/resources/1", GetFormattedPath("(root).metrics.metrics_collected.cpu.resources.1"))
	assert.Equal(t, "/metrics/metrics_collected/cpu", GetFormattedPath("(root).metrics.metrics_collected.cpu"))
}
