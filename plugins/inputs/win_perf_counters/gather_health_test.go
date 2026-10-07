// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package win_perf_counters

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrapeOutcomeAllFailed(t *testing.T) {
	err := errors.New("pdh init failed")
	assert.False(t, scrapeOutcome{}.allFailed())
	assert.False(t, scrapeOutcome{total: 2, failed: 1, lastErr: err}.allFailed())
	assert.False(t, scrapeOutcome{total: 2, failed: 2}.allFailed()) // missing lastErr
	assert.True(t, scrapeOutcome{total: 2, failed: 2, lastErr: err}.allFailed())
}

func TestScrapeOutcomeError(t *testing.T) {
	err := errors.New("pdh init failed")
	assert.NoError(t, scrapeOutcome{total: 1, failed: 0}.error())
	got := scrapeOutcome{total: 3, failed: 3, lastErr: err}.error()
	require.Error(t, got)
	assert.ErrorIs(t, got, err)
	assert.Contains(t, got.Error(), "all 3 counters failed")
}

func TestNextFailureCount(t *testing.T) {
	err := errors.New("pdh")
	fail := scrapeOutcome{total: 1, failed: 1, lastErr: err}
	ok := scrapeOutcome{total: 1, failed: 0}

	assert.Equal(t, 1, nextFailureCount(0, fail))
	assert.Equal(t, 4, nextFailureCount(3, fail))
	assert.Equal(t, 0, nextFailureCount(4, ok))
}

func TestFailureLogPrefix(t *testing.T) {
	assert.Equal(t, "", failureLogPrefix(0))
	assert.Equal(t, "W!", failureLogPrefix(1))
	assert.Equal(t, "", failureLogPrefix(2))
	assert.Equal(t, "E!", failureLogPrefix(initFailureEscalateAfter))
	assert.Equal(t, "E!", failureLogPrefix(initFailureResyncEvery))
	assert.Equal(t, "E!", failureLogPrefix(20))
	assert.Equal(t, "", failureLogPrefix(11))
}
