// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

func TestEc2TaggerTranslator(t *testing.T) {
	tt := newEc2TaggerTranslator()
	assert.Equal(t, "ec2tagger/"+common.PipelineNameSystemMetrics, tt.ID().String())

	got, err := tt.Translate(nil)
	require.NoError(t, err)
	cfg, ok := got.(*ec2tagger.Config)
	require.True(t, ok)
	assert.Equal(t, []string{"InstanceId"}, cfg.EC2MetadataTags)
	assert.Equal(t, &agenthealth.StatusCodeID, cfg.MiddlewareID)
	assert.Greater(t, cfg.IMDSRetries, 0)
}
