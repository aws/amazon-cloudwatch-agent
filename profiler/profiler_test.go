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
	sort.Sort(sort.StringSlice(stats))
	sort.Sort(sort.StringSlice(output))

	assert.True(t, len(Profiler.stats) == 0)
	assert.Equal(t, stats, output, "Stats do not match")

	output = Profiler.reportAndClear()
	noStats := []string{
		noStatsInProfiler,
	}
	sort.Sort(sort.StringSlice(noStats))
	sort.Sort(sort.StringSlice(output))

	assert.True(t, len(Profiler.stats) == 0)
	assert.Equal(t, noStats, output, "Stats do not match")
}
