// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlagSet(t *testing.T) {
	fs := &flagSet{}
	var notifyCount int
	fs.OnChange(func() {
		notifyCount++
	})
	assert.False(t, fs.IsSet(FlagIMDSFallbackSuccess))
	assert.Nil(t, fs.GetString(FlagIMDSFallbackSuccess))
	fs.Set(FlagIMDSFallbackSuccess)
	assert.True(t, fs.IsSet(FlagIMDSFallbackSuccess))
	assert.Nil(t, fs.GetString(FlagIMDSFallbackSuccess))
	assert.Equal(t, 1, notifyCount)
	// already set, so ignored
	fs.SetValue(FlagIMDSFallbackSuccess, "ignores this")
	assert.Nil(t, fs.GetString(FlagIMDSFallbackSuccess))
	assert.Equal(t, 1, notifyCount)
	fs.SetValues(map[Flag]any{
		FlagMode:       "test/mode",
		FlagRegionType: "test/region-type",
	})
	assert.True(t, fs.IsSet(FlagMode))
	assert.True(t, fs.IsSet(FlagRegionType))
	got := fs.GetString(FlagMode)
	assert.NotNil(t, got)
	assert.Equal(t, "test/mode", *got)
	got = fs.GetString(FlagRegionType)
	assert.NotNil(t, got)
	assert.Equal(t, "test/region-type", *got)
	assert.Equal(t, 2, notifyCount)
	fs.SetValues(map[Flag]any{
		FlagRegionType: "other",
	})
	assert.NotNil(t, got)
	assert.Equal(t, "test/region-type", *got)
	assert.Equal(t, 2, notifyCount)
	fs.SetValues(map[Flag]any{
		FlagMode:               "other/mode",
		FlagRunningInContainer: true,
	})
	got = fs.GetString(FlagMode)
	assert.NotNil(t, got)
	assert.Equal(t, "test/mode", *got)
	assert.True(t, fs.IsSet(FlagRunningInContainer))
	assert.Equal(t, 3, notifyCount)
}

func TestFlag(t *testing.T) {
	testCases := []struct {
		flag Flag
		str  string
	}{
		{flag: FlagAppSignal, str: flagAppSignalsStr},
		{flag: FlagEnhancedContainerInsights, str: flagEnhancedContainerInsightsStr},
		{flag: FlagIMDSFallbackSuccess, str: flagIMDSFallbackSuccessStr},
		{flag: FlagMode, str: flagModeStr},
		{flag: FlagRegionType, str: flagRegionTypeStr},
		{flag: FlagRunningInContainer, str: flagRunningInContainerStr},
		{flag: FlagSharedConfigFallback, str: flagSharedConfigFallbackStr},
	}
	for _, testCase := range testCases {
		flag := testCase.flag
		got, err := flag.MarshalText()
		assert.NoError(t, err)
		assert.EqualValues(t, testCase.str, got)
		assert.NoError(t, flag.UnmarshalText(got))
		assert.Equal(t, flag, testCase.flag)
	}
}

func TestInvalidFlag(t *testing.T) {
	f := Flag(-1)
	got, err := f.MarshalText()
	assert.Error(t, err)
	assert.ErrorIs(t, err, errUnsupportedFlag)
	assert.Nil(t, got)
	err = f.UnmarshalText([]byte("Flag(-1)"))
	assert.Error(t, err)
	assert.ErrorIs(t, err, errUnsupportedFlag)
}
