// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package headerssetter

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestTranslatorID(t *testing.T) {
	tr := NewTranslatorWithName("test_logs")
	assert.Equal(t, "headers_setter/test_logs", tr.ID().String())
}

func TestTranslatorWithHeaders(t *testing.T) {
	headers := []HeaderMapping{
		{HeaderName: "x-aws-log-group", ContextKey: "aws.log.group.name"},
		{HeaderName: "x-aws-log-stream", Value: "my-stream"},
	}
	tr := NewTranslatorWithName("logs", WithHeaders(headers))

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	hsCfg := cfg.(*headerssetterextension.Config)
	require.Len(t, hsCfg.HeadersConfig, 2)

	assert.Equal(t, "x-aws-log-group", *hsCfg.HeadersConfig[0].Key)
	assert.Equal(t, "aws.log.group.name", *hsCfg.HeadersConfig[0].FromContext)
	assert.Nil(t, hsCfg.HeadersConfig[0].Value)
	assert.Equal(t, headerssetterextension.UPSERT, hsCfg.HeadersConfig[0].Action)

	assert.Equal(t, "x-aws-log-stream", *hsCfg.HeadersConfig[1].Key)
	assert.Nil(t, hsCfg.HeadersConfig[1].FromContext)
	assert.Equal(t, "my-stream", *hsCfg.HeadersConfig[1].Value)
	assert.Equal(t, headerssetterextension.UPSERT, hsCfg.HeadersConfig[1].Action)
}

func TestTranslatorWithAdditionalAuth(t *testing.T) {
	authID := component.MustNewID("awscloudwatchlogsprovisioner")
	tr := NewTranslatorWithName("logs", WithAdditionalAuth(authID))

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	hsCfg := cfg.(*headerssetterextension.Config)
	require.NotNil(t, hsCfg.AdditionalAuth)
	assert.Equal(t, authID, *hsCfg.AdditionalAuth)
}

func TestTranslatorNoOptions(t *testing.T) {
	tr := NewTranslatorWithName("empty")

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	hsCfg := cfg.(*headerssetterextension.Config)
	assert.Nil(t, hsCfg.AdditionalAuth)
	assert.Empty(t, hsCfg.HeadersConfig)
}
