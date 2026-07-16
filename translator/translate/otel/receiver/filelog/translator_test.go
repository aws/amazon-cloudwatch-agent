// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filelog

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
)

func TestTranslator_ID(t *testing.T) {
	tr := NewTranslator(WithNamePrefix("postgresql"), WithIndex(0))
	assert.Equal(t, component.MustNewIDWithName("filelog", "postgresql_0"), tr.ID())
}

func TestTranslator_ID_WithName(t *testing.T) {
	tr := NewTranslator(WithName("my_custom_name"))
	assert.Equal(t, component.MustNewIDWithName("filelog", "my_custom_name"), tr.ID())
}

func TestTranslator_Translate(t *testing.T) {
	tr := NewTranslator(
		WithFilePath("/var/log/postgresql/postgresql.log"),
		WithIndex(0),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	flCfg := cfg.(*filelogreceiver.FileLogConfig)
	assert.Equal(t, []string{"/var/log/postgresql/postgresql.log"}, flCfg.InputConfig.Include)
	assert.Equal(t, "end", flCfg.InputConfig.StartAt)
	assert.Equal(t, "utf-8", flCfg.InputConfig.Encoding)
}

func TestTranslator_WithEncoding(t *testing.T) {
	tr := NewTranslator(
		WithFilePath("/var/log/app.log"),
		WithIndex(0),
		WithEncoding("utf-16"),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	flCfg := cfg.(*filelogreceiver.FileLogConfig)
	assert.Equal(t, "utf-16", flCfg.InputConfig.Encoding)
}

func TestTranslator_WithMultilinePattern(t *testing.T) {
	tr := NewTranslator(
		WithFilePath("/var/log/app.log"),
		WithIndex(0),
		WithMultilinePattern(`^\d{4}-\d{2}-\d{2}`),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	flCfg := cfg.(*filelogreceiver.FileLogConfig)
	assert.Equal(t, `^\d{4}-\d{2}-\d{2}`, flCfg.InputConfig.SplitConfig.LineStartPattern)
}

func TestTranslator_WithStorage(t *testing.T) {
	tr := NewTranslator(
		WithFilePath("/var/log/app.log"),
		WithIndex(0),
		WithStorage(),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	flCfg := cfg.(*filelogreceiver.FileLogConfig)
	require.NotNil(t, flCfg.StorageID)
	assert.Equal(t, filestorage.ComponentID(), *flCfg.StorageID)
}

func TestTranslator_WithResource(t *testing.T) {
	tr := NewTranslator(
		WithFilePath("/var/log/app.log"),
		WithIndex(0),
		WithResource(map[string]string{
			"aws.log.source": "files",
		}),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	flCfg := cfg.(*filelogreceiver.FileLogConfig)
	assert.True(t, flCfg.InputConfig.IncludeFileName)
	assert.Contains(t, flCfg.InputConfig.Resource, "aws.log.source")
}

func TestTranslator_WithStartAtBeginning(t *testing.T) {
	tr := NewTranslator(
		WithFilePath("/var/log/app.log"),
		WithIndex(0),
		WithStartAtBeginning(),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	flCfg := cfg.(*filelogreceiver.FileLogConfig)
	assert.Equal(t, "beginning", flCfg.InputConfig.StartAt)
}

func TestTranslator_WithTimestampFormat_ReturnsRawMapConfig(t *testing.T) {
	tr := NewTranslator(
		WithFilePath("/var/log/app.log"),
		WithName("test_receiver"),
		WithTimestampFormat("%Y-%m-%d %H:%M:%S", "UTC"),
		WithStorage(),
		WithResource(map[string]string{
			"aws.log.source": "files",
		}),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	raw, ok := cfg.(*rawMapConfig)
	require.True(t, ok, "expected rawMapConfig when timestamp format is set")

	assert.Equal(t, []string{"/var/log/app.log"}, raw.data["include"])
	assert.Equal(t, "end", raw.data["start_at"])
	assert.Equal(t, "utf-8", raw.data["encoding"])
	assert.Equal(t, filestorage.ComponentID().String(), raw.data["storage"])

	resource, ok := raw.data["resource"].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "files", resource["aws.log.source"])

	operators, ok := raw.data["operators"].([]any)
	require.True(t, ok)
	require.Len(t, operators, 1)

	op := operators[0].(map[string]any)
	assert.Equal(t, "regex_parser", op["type"])
	assert.Contains(t, op["regex"], "(?P<timestamp>")

	ts := op["timestamp"].(map[string]any)
	assert.Equal(t, "2006-1-_2 15:04:05", ts["layout"])
	assert.Equal(t, "gotime", ts["layout_type"])
	assert.Equal(t, "UTC", ts["location"])
}

func TestTranslator_WithTimestampFormat_LocalTimezone(t *testing.T) {
	tr := NewTranslator(
		WithFilePath("/var/log/app.log"),
		WithName("test_receiver"),
		WithTimestampFormat("%Y-%m-%d %H:%M:%S", "Local"),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	raw := cfg.(*rawMapConfig)
	operators := raw.data["operators"].([]any)
	op := operators[0].(map[string]any)
	ts := op["timestamp"].(map[string]any)
	assert.Equal(t, "Local", ts["location"])
}

func TestRawMapConfig_Marshal(t *testing.T) {
	raw := &rawMapConfig{data: map[string]any{
		"include":  []string{"/var/log/test.log"},
		"start_at": "end",
	}}

	conf := confmap.New()
	err := raw.Marshal(conf)
	require.NoError(t, err)

	assert.Equal(t, "end", conf.Get("start_at"))
}
