// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFlagStats(t *testing.T) {
	provider := newFlagStats(time.Microsecond)
	got := provider.stats
	assert.Nil(t, got.ImdsFallbackSucceed)
	assert.Nil(t, got.SharedConfigFallback)
	provider.SetFlag(FlagIMDSFallbackSucceed)
	assert.Nil(t, got.ImdsFallbackSucceed)
	got = provider.stats
	assert.NotNil(t, got.ImdsFallbackSucceed)
	assert.Equal(t, 1, *got.ImdsFallbackSucceed)
	assert.Nil(t, got.SharedConfigFallback)
	provider.SetFlag(FlagSharedConfigFallback)
	got = provider.stats
	assert.NotNil(t, got.SharedConfigFallback)
	assert.Equal(t, 1, *got.SharedConfigFallback)
}
