// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestFlagStats(t *testing.T) {
	t.Setenv(envconfig.RunInContainer, envconfig.TrueValue)
	provider := newFlagStats(time.Microsecond)
	got := provider.getStats()
	assert.Nil(t, got.ImdsFallbackSucceed)
	assert.Nil(t, got.SharedConfigFallback)
	assert.NotNil(t, got.RunningInContainer)
	assert.Equal(t, 1, *got.RunningInContainer)
	provider.SetFlag(FlagIMDSFallbackSucceed)
	assert.Nil(t, got.ImdsFallbackSucceed)
	got = provider.getStats()
	assert.NotNil(t, got.ImdsFallbackSucceed)
	assert.Equal(t, 1, *got.ImdsFallbackSucceed)
	assert.Nil(t, got.SharedConfigFallback)
	provider.SetFlag(FlagSharedConfigFallback)
	got = provider.getStats()
	assert.NotNil(t, got.SharedConfigFallback)
	assert.Equal(t, 1, *got.SharedConfigFallback)
	provider.SetFlagWithValue(FlagMode, "test")
	got = provider.getStats()
	assert.NotNil(t, got.Mode)
	assert.Equal(t, "test", *got.Mode)
}
