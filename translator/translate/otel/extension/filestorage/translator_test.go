// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package filestorage

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	assert.Equal(t, "file_storage/journald", tt.ID().String())

	conf := confmap.NewFromStringMap(map[string]interface{}{})
	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	gotCfg, ok := got.(*filestorage.Config)
	require.True(t, ok)
	assert.Equal(t, "/opt/aws/amazon-cloudwatch-agent/logs/state", gotCfg.Directory)
	assert.True(t, gotCfg.CreateDirectory)
}
