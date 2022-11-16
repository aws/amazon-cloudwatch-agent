// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package profiler

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProfiler(t *testing.T) {
	Profiler.AddStats([]string{"pluginA", "StatsA"}, 0.0)
	Profiler.AddStats([]string{"pluginB", "StatsB"}, 0.1)
	output := Profiler.reportAndClear()

	stats := []string{
		"[pluginB_StatsB: 0.100000]",
		"[pluginA_StatsA: 0.000000]",
	}
	sort.Strings(stats)
	sort.Strings(output)

	assert.True(t, len(Profiler.stats) == 0)
	assert.Equal(t, stats, output, "Stats do not match")

	output = Profiler.reportAndClear()
	noStats := []string{
		noStatsInProfiler,
	}
	sort.Strings(noStats)
	sort.Strings(output)

	assert.True(t, len(Profiler.stats) == 0)
	assert.Equal(t, noStats, output, "Stats do not match")
}

func TestProfilerGetStats(t *testing.T) {
	Profiler.AddStats([]string{t.Name(), "StatsA"}, 0.0)

	var val float64
	var ok bool

	stats := Profiler.GetStats()
	name := t.Name() + "_StatsA"
	if val, ok = stats[name]; !ok {
		t.Errorf("%s was not found in the stats map", name)
	}
	assert.Equal(t, 0.0, val)

	Profiler.ReportAndClear()
	stats = Profiler.GetStats()
	_, ok = stats[name]
	assert.False(t, ok)
}
