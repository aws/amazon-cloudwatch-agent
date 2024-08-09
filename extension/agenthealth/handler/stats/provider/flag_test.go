// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

func TestFlagStats(t *testing.T) {
	t.Skip("stat provider tests are flaky. disable until fix is available")
	t.Setenv(envconfig.RunInContainer, envconfig.TrueValue)
	fs := newFlagStats(agent.UsageFlags(), time.Microsecond)
	got := fs.getStats()
	assert.Nil(t, got.ImdsFallbackSucceed)
	assert.Nil(t, got.SharedConfigFallback)
	assert.NotNil(t, got.RunningInContainer)
	assert.Equal(t, 1, *got.RunningInContainer)
	fs.flagSet.Set(agent.FlagIMDSFallbackSuccess)
	assert.Nil(t, got.ImdsFallbackSucceed)
	got = fs.getStats()
	assert.NotNil(t, got.ImdsFallbackSucceed)
	assert.Equal(t, 1, *got.ImdsFallbackSucceed)
	assert.Nil(t, got.SharedConfigFallback)
	fs.flagSet.Set(agent.FlagSharedConfigFallback)
	got = fs.getStats()
	assert.NotNil(t, got.SharedConfigFallback)
	assert.Equal(t, 1, *got.SharedConfigFallback)
	fs.flagSet.SetValue(agent.FlagMode, "test")
	got = fs.getStats()
	assert.NotNil(t, got.Mode)
	assert.Equal(t, "test", *got.Mode)
}
