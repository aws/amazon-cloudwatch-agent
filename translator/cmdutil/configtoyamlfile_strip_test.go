// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Verifies ConfigToYamlFile drops a null sending_queue.batch but keeps a configured one.
func TestConfigToYamlFileStripsNullSendingQueueBatch(t *testing.T) {
	cfg := map[string]any{
		"exporters": map[string]any{
			"otlp_http/profiles": map[string]any{
				"sending_queue": map[string]any{
					"batch":         nil,
					"queue_size":    100,
					"num_consumers": 10,
				},
			},
			"otlp_http/custom": map[string]any{
				"sending_queue": map[string]any{
					"batch": map[string]any{"flush_timeout": "200ms"},
				},
			},
			"awsemf/emf_logs": map[string]any{
				"region": "us-east-1",
			},
		},
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, ConfigToYamlFile(cfg, path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, yaml.Unmarshal(data, &got))

	exporters := got["exporters"].(map[string]any)

	profilesQueue := exporters["otlp_http/profiles"].(map[string]any)["sending_queue"].(map[string]any)
	_, hasBatch := profilesQueue["batch"]
	require.False(t, hasBatch, "null batch should be stripped")
	require.EqualValues(t, 100, profilesQueue["queue_size"], "queue_size preserved")

	customQueue := exporters["otlp_http/custom"].(map[string]any)["sending_queue"].(map[string]any)
	require.NotNil(t, customQueue["batch"], "a configured batch must be preserved")

	require.Equal(t, "us-east-1", exporters["awsemf/emf_logs"].(map[string]any)["region"], "exporter without sending_queue untouched")
}
