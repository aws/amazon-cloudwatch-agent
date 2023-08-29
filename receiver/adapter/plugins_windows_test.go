// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	_ "github.com/aws/amazon-cloudwatch-agent/plugins/inputs/win_perf_counters"
)

func Test_WindowsPerfCountersPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		testConfig:         "./testdata/windows_plugins.toml",
		plugin:             "win_perf_counters",
		regularInputConfig: regularInputConfig{scrapeCount: 1},
		serviceInputConfig: serviceInputConfig{},
		// This unit test is relying on the host to actually have these perf counters.
		// Observed an issue where some perf counters were intermittently retrievable.
		// Therefore we choose 2 perf counters which seem to be available consistently.

		expectedMetrics:      [][]string{{"Memory % Committed Bytes In Use"}},
		numMetricsComparator: assert.Equal,
	})
}
