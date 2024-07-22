// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

func TestIntervalStats(t *testing.T) {
	t.Skip("stat provider tests are flaky. disable until fix is available")
	s := newIntervalStats(time.Millisecond)
	s.stats.Store(agent.Stats{
		ThreadCount: aws.Int32(2),
	})
	assert.NotNil(t, s.Stats("").ThreadCount)
	assert.Nil(t, s.Stats("").ThreadCount)
	time.Sleep(time.Millisecond)
	assert.Eventually(t, func() bool {
		return s.Stats("").ThreadCount != nil
	}, 5*time.Millisecond, time.Millisecond)
}
