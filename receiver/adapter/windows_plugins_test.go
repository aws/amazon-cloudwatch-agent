//go:build windows
// +build windows

package adapter

import (
	"log"
	"testing"

	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/win_perf_counters"
	"github.com/stretchr/testify/assert"
)

func Test_WindowsPerfCountersPlugin(t *testing.T) {
	log.Printf("windows perf counter plugin started")
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		testCfg:            "./testdata/windows_plugins.toml",
		plugin:             "win_perf_counters",
		regularInputConfig: regularInputConfig{scrapeCount: 1},
		serviceInputConfig: serviceInputConfig{},

		expectedMetrics:      [][]string{{"LogicalDisk % Free Space"}, {"Memory % Committed Bytes In Use"}},
		numMetricsComparator: assert.Equal,
	})
}
