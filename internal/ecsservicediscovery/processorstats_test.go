package ecsservicediscovery

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_ProcessorStats_Normal(t *testing.T) {
	var stats ProcessorStats
	stats.AddStats("test1")
	stats.AddStats("test1")
	stats.AddStatsCount("stats_count", 100)
	stats.AddStatsCount("stats_count", 200)
	stats.ShowStats()

	assert.Equal(t, 2, stats.GetStats("test1"))
	assert.Equal(t, 300, stats.GetStats("stats_count"))
	assert.Equal(t, 0, stats.GetStats("stats_wrong"))
}
