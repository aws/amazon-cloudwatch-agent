//go:build windows
// +build windows

package adapter

import (
	"testing"

	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/win_perf_counters"
	"github.com/stretchr/testify/assert"
)

func Test_WindowsPerfCountersPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		testCfg:            "./testdata/windows_plugins.toml",
		plugin:             "win_perf_counters",
		regularInputConfig: regularInputConfig{scrapeCount: 1},
		serviceInputConfig: serviceInputConfig{},

		expectedMetrics:      [][]string{{"Memory % Committed Bytes In Use"}, {"TCPv4 Connections Established"}},
		numMetricsComparator: assert.Equal,
	})
}
