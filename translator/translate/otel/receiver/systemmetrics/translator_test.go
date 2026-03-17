// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/receiver/systemmetricsreceiver"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	assert.Equal(t, "systemmetrics", tt.ID().String())

	got, err := tt.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	cfg, ok := got.(*systemmetricsreceiver.Config)
	require.True(t, ok)
	assert.Equal(t, 60*time.Second, cfg.CollectionInterval)
}
