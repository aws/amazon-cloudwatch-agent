// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filter

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFromConfig(t *testing.T) {
	logger := slog.Default()

	testCases := map[string]struct {
		config Config
		want   func(t *testing.T, filters Filters)
	}{
		"EmptyConfig": {
			config: Config{},
			want: func(t *testing.T, filters Filters) {
				assert.Nil(t, filters.Process.Pre)
				assert.Nil(t, filters.Process.Name)
			},
		},
		"MinUptimeOnly": {
			config: Config{
				Process: ProcessConfig{
					MinUptime: time.Minute,
				},
			},
			want: func(t *testing.T, filters Filters) {
				assert.NotNil(t, filters.Process.Pre)
				assert.Nil(t, filters.Process.Name)
			},
		},
		"ExcludeNamesOnly": {
			config: Config{
				Process: ProcessConfig{
					ExcludeNames: []string{"test"},
				},
			},
			want: func(t *testing.T, filters Filters) {
				assert.Nil(t, filters.Process.Pre)
				assert.NotNil(t, filters.Process.Name)
			},
		},
		"BothFilters": {
			config: Config{
				Process: ProcessConfig{
					MinUptime:    time.Minute,
					ExcludeNames: []string{"test"},
				},
			},
			want: func(t *testing.T, filters Filters) {
				assert.NotNil(t, filters.Process.Pre)
				assert.NotNil(t, filters.Process.Name)
			},
		},
		"MinUptime/Zero": {
			config: Config{
				Process: ProcessConfig{
					MinUptime: 0,
				},
			},
			want: func(t *testing.T, filters Filters) {
				assert.Nil(t, filters.Process.Pre)
			},
		},
		"MinUptime/Negative": {
			config: Config{
				Process: ProcessConfig{
					MinUptime: -time.Minute,
				},
			},
			want: func(t *testing.T, filters Filters) {
				assert.Nil(t, filters.Process.Pre)
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			filters := FromConfig(logger, testCase.config)
			testCase.want(t, filters)
		})
	}
}
