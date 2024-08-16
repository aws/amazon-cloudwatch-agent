// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"testing"

	"github.com/influxdata/telegraf/config"
	"github.com/stretchr/testify/assert"
)

func TestRetentionAlreadySet(t *testing.T) {
	c := config.NewConfig()
	l := NewLogAgent(c)
	assert.False(t, l.retentionAlreadyAttempted["logGroup1"])
	firstAttempt := l.checkRetentionAlreadyAttempted(3, "logGroup1")
	assert.Equal(t, 3, firstAttempt)
	secondAttempt := l.checkRetentionAlreadyAttempted(3, "logGroup1")
	assert.Equal(t, -1, secondAttempt)
	assert.True(t, l.retentionAlreadyAttempted["logGroup1"])
}
