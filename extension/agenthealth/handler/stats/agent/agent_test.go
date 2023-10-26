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
		CpuPercent:                aws.Float64(1.5),
		MemoryBytes:               aws.Uint64(133),
		FileDescriptorCount:       aws.Int32(456),
		ThreadCount:               aws.Int32(789),
		LatencyMillis:             aws.Int64(1234),
		PayloadBytes:              aws.Int(5678),
		StatusCode:                aws.Int(200),
		SharedConfigFallback:      aws.Int(1),
		ImdsFallbackSucceed:       aws.Int(1),
		AppSignals:                aws.Int(1),
		EnhancedContainerInsights: aws.Int(1),
		RunningInContainer:        aws.Int(0),
		RegionType:                aws.String("RegionType"),
		Mode:                      aws.String("Mode"),
	})
	assert.EqualValues(t, 1.5, *stats.CpuPercent)
	assert.EqualValues(t, 133, *stats.MemoryBytes)
	assert.EqualValues(t, 456, *stats.FileDescriptorCount)
	assert.EqualValues(t, 789, *stats.ThreadCount)
	assert.EqualValues(t, 1234, *stats.LatencyMillis)
	assert.EqualValues(t, 5678, *stats.PayloadBytes)
	assert.EqualValues(t, 200, *stats.StatusCode)
	assert.EqualValues(t, 1, *stats.ImdsFallbackSucceed)
	assert.EqualValues(t, 1, *stats.SharedConfigFallback)
	assert.EqualValues(t, 1, *stats.AppSignals)
	assert.EqualValues(t, 1, *stats.EnhancedContainerInsights)
	assert.EqualValues(t, 0, *stats.RunningInContainer)
	assert.EqualValues(t, "RegionType", *stats.RegionType)
	assert.EqualValues(t, "Mode", *stats.Mode)
}

func TestMarshal(t *testing.T) {
	testCases := map[string]struct {
		stats *Stats
		want  string
	}{
		"WithEmpty": {
			stats: &Stats{},
			want:  "",
		},
		"WithPartial": {
			stats: &Stats{
				CpuPercent:   aws.Float64(1.2),
				MemoryBytes:  aws.Uint64(123),
				ThreadCount:  aws.Int32(789),
				PayloadBytes: aws.Int(5678),
			},
			want: `"cpu":1.2,"mem":123,"th":789,"load":5678`,
		},
		"WithFull": {
			stats: &Stats{
				CpuPercent:          aws.Float64(1.2),
				MemoryBytes:         aws.Uint64(123),
				FileDescriptorCount: aws.Int32(456),
				ThreadCount:         aws.Int32(789),
				LatencyMillis:       aws.Int64(1234),
				PayloadBytes:        aws.Int(5678),
				StatusCode:          aws.Int(200),
				ImdsFallbackSucceed: aws.Int(1),
			},
			want: `"cpu":1.2,"mem":123,"fd":456,"th":789,"lat":1234,"load":5678,"code":200,"ifs":1`,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := testCase.stats.Marshal()
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestOperationFilter(t *testing.T) {
	testCases := map[string]struct {
		allowedOperations []string
		testOperations    []string
		want              []bool
	}{
		"WithNoneAllowed": {allowedOperations: nil, testOperations: []string{"nothing", "is", "allowed"}, want: []bool{false, false, false}},
		"WithSomeAllowed": {allowedOperations: []string{"are"}, testOperations: []string{"some", "are", "allowed"}, want: []bool{false, true, false}},
		"WithAllAllowed":  {allowedOperations: []string{"*"}, testOperations: []string{"all", "are", "allowed"}, want: []bool{true, true, true}},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			filter := NewOperationsFilter(testCase.allowedOperations...)
			for index, testOperation := range testCase.testOperations {
				assert.Equal(t, testCase.want[index], filter.IsAllowed(testOperation))
			}
		})
	}
}
