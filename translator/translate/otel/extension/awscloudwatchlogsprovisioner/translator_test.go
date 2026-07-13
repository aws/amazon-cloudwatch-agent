// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscloudwatchlogsprovisioner

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscloudwatchlogsprovisionerextension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

func TestTranslatorID(t *testing.T) {
	authID := component.NewIDWithName(component.MustNewType("sigv4auth"), "test")
	tr := NewTranslator(authID)
	assert.Equal(t, "awscloudwatchlogsprovisioner", tr.ID().String())
}

func TestTranslatorTranslate(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	authID := component.NewIDWithName(component.MustNewType("sigv4auth"), "test")
	tr := NewTranslator(authID)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	provCfg := cfg.(*awscloudwatchlogsprovisionerextension.Config)
	assert.Equal(t, "us-west-2", provCfg.Region)
	assert.Equal(t, &authID, provCfg.AdditionalAuth)
}

func TestTranslatorTranslateNoRegion(t *testing.T) {
	agent.Global_Config.Region = ""
	authID := component.NewIDWithName(component.MustNewType("sigv4auth"), "test")
	tr := NewTranslator(authID)

	_, err := tr.Translate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "region is required")
}
