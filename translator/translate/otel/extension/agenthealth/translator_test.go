// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

func TestTranslate(t *testing.T) {
	operations := []string{OperationPutLogEvents}
	tt := NewTranslator("test", operations).(*translator)
	assert.Equal(t, "agenthealth/test", tt.ID().String())
	tt.isUsageDataEnabled = true
	got, err := tt.Translate(nil)
	assert.NoError(t, err)
	assert.Equal(t, &agenthealth.Config{IsUsageDataEnabled: true, Stats: agent.StatsConfig{Operations: operations}}, got)
	tt.isUsageDataEnabled = false
	got, err = tt.Translate(nil)
	assert.NoError(t, err)
	assert.Equal(t, &agenthealth.Config{IsUsageDataEnabled: false, Stats: agent.StatsConfig{Operations: operations}}, got)
}
