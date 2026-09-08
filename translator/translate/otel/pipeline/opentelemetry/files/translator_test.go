// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pipeline"
)

func TestFilesPipelineTranslator_ID(t *testing.T) {
	translator := &filesPipelineTranslator{
		entry: fileEntry{
			index:    0,
			filePath: "/var/log/app.log",
		},
	}
	expected := pipeline.NewIDWithName(pipeline.SignalLogs, "files__var_log_app_log_0")
	assert.Equal(t, expected, translator.ID())
}

func TestFilesPipelineTranslator_Translate_WithRouting(t *testing.T) {
	translator := &filesPipelineTranslator{
		entry: fileEntry{
			index:         0,
			filePath:      "/var/log/app.log",
			encoding:      "utf-8",
			logGroupName:  "/aws/app",
			logStreamName: "{hostname}",
			resource: map[string]string{
				"aws.log.source": "files",
			},
		},
	}

	result, err := translator.Translate(nil)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Receivers.Len())
	assert.Equal(t, 3, result.Processors.Len()) // resource processor + groupbyattrs + scope transform
	assert.Equal(t, 1, result.Exporters.Len())
	assert.Equal(t, 1, result.Extensions.Len())
	assert.Equal(t, 1, result.Connectors.Len())
}

func TestFilesPipelineTranslator_Translate_WithoutRouting(t *testing.T) {
	translator := &filesPipelineTranslator{
		entry: fileEntry{
			index:    0,
			filePath: "/var/log/app.log",
			encoding: "utf-8",
			resource: map[string]string{
				"aws.log.source": "files",
			},
		},
	}

	result, err := translator.Translate(nil)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Receivers.Len())
	assert.Equal(t, 2, result.Processors.Len()) // groupbyattrs + scope transform
	assert.Equal(t, 1, result.Exporters.Len())
	assert.Equal(t, 1, result.Extensions.Len())
	assert.Equal(t, 1, result.Connectors.Len())
}

func TestFilesPipelineTranslator_Translate_WithTimestamp(t *testing.T) {
	translator := &filesPipelineTranslator{
		entry: fileEntry{
			index:           0,
			filePath:        "/var/log/app.log",
			encoding:        "utf-8",
			timestampFormat: "%Y-%m-%d %H:%M:%S",
			timezone:        "UTC",
			resource: map[string]string{
				"aws.log.source": "files",
			},
		},
	}

	result, err := translator.Translate(nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Receivers.Len())
}

func TestFilesPipelineTranslator_Translate_WithMultiline(t *testing.T) {
	translator := &filesPipelineTranslator{
		entry: fileEntry{
			index:            0,
			filePath:         "/var/log/app.log",
			encoding:         "utf-8",
			multilinePattern: `^\d{4}-\d{2}-\d{2}`,
			resource: map[string]string{
				"aws.log.source": "files",
			},
		},
	}

	result, err := translator.Translate(nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Receivers.Len())
}

func TestFilesPipelineTranslator_Translate_TimestampFormatMagicWithoutFormat(t *testing.T) {
	translator := &filesPipelineTranslator{
		entry: fileEntry{
			index:            0,
			filePath:         "/var/log/app.log",
			encoding:         "utf-8",
			multilinePattern: "{timestamp_format}",
			resource: map[string]string{
				"aws.log.source": "files",
			},
		},
	}

	_, err := translator.Translate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timestamp_format is not set")
}
