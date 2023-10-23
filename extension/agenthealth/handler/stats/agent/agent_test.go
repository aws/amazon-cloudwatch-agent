// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
)

func TestMerge(t *testing.T) {
	stats := &Stats{CpuPercent: aws.Float64(1.2)}
	assert.EqualValues(t, 1.2, *stats.CpuPercent)
	assert.Nil(t, stats.MemoryBytes)
	stats.Merge(Stats{
		CpuPercent:  aws.Float64(1.3),
		MemoryBytes: aws.Uint64(123),
	})
	assert.EqualValues(t, 1.3, *stats.CpuPercent)
	assert.EqualValues(t, 123, *stats.MemoryBytes)
	stats.Merge(Stats{
		FileDescriptorCount:  aws.Int32(456),
		ThreadCount:          aws.Int32(789),
		LatencyMillis:        aws.Int64(1234),
		PayloadBytes:         aws.Int(5678),
		StatusCode:           aws.Int(200),
		ImdsFallbackSucceed:  aws.Int(1),
		SharedConfigFallback: aws.Int(1),
	})
	assert.EqualValues(t, 1.3, *stats.CpuPercent)
	assert.EqualValues(t, 123, *stats.MemoryBytes)
	assert.EqualValues(t, 456, *stats.FileDescriptorCount)
	assert.EqualValues(t, 789, *stats.ThreadCount)
	assert.EqualValues(t, 1234, *stats.LatencyMillis)
	assert.EqualValues(t, 5678, *stats.PayloadBytes)
	assert.EqualValues(t, 200, *stats.StatusCode)
	assert.EqualValues(t, 1, *stats.ImdsFallbackSucceed)
	assert.EqualValues(t, 1, *stats.SharedConfigFallback)
}
