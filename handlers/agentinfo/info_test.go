// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentinfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
)

func TestNew(t *testing.T) {
	ai := New("")
	expectedUserAgentRegex := `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$`

	assert.Regexp(t, regexp.MustCompile(expectedUserAgentRegex), ai.UserAgent())
}

func TestRecordOpData(t *testing.T) {
	ai := newAgentInfo("")

	stats := ai.StatsHeader()
	actual := agentStats{}
	require.NoError(t, json.Unmarshal([]byte("{"+stats+"}"), &actual))
	assert.Nil(t, actual.LatencyMillis)
	assert.Nil(t, actual.PayloadBytes)
	assert.Nil(t, actual.StatusCode)

	ai.RecordOpData(100000000, 10, nil)
	stats = ai.StatsHeader()
	actual = agentStats{}
	require.NoError(t, json.Unmarshal([]byte("{"+stats+"}"), &actual))
	assert.EqualValues(t, 100, *actual.LatencyMillis)
	assert.EqualValues(t, 10, *actual.PayloadBytes)
	assert.EqualValues(t, http.StatusOK, *actual.StatusCode)

	ai.RecordOpData(200000000, 20, errors.New(""))
	stats = ai.StatsHeader()
	actual = agentStats{}
	require.NoError(t, json.Unmarshal([]byte("{"+stats+"}"), &actual))
	assert.EqualValues(t, 200, *actual.LatencyMillis)
	assert.EqualValues(t, 20, *actual.PayloadBytes)
	assert.Nil(t, actual.StatusCode)

	ai.RecordOpData(300000000, 30, awserr.NewRequestFailure(awserr.New("", "", errors.New("")), 500, ""))
	stats = ai.StatsHeader()
	actual = agentStats{}
	require.NoError(t, json.Unmarshal([]byte("{"+stats+"}"), &actual))
	assert.EqualValues(t, 300, *actual.LatencyMillis)
	assert.EqualValues(t, 30, *actual.PayloadBytes)
	assert.EqualValues(t, 500, *actual.StatusCode)
}

func TestSetComponents(t *testing.T) {
	otelcfg := &otelcol.Config{
		Service: service.Config{
			Pipelines: map[component.ID]*service.PipelineConfig{
				component.NewID("metrics"): {
					Receivers: []component.ID{
						component.NewID(adapter.TelegrafPrefix + "cpu"),
						component.NewID("prometheus"),
					},
					Processors: []component.ID{
						component.NewID("batch"),
						component.NewID("filter"),
					},
					Exporters: []component.ID{
						component.NewID("cloudwatch"),
					},
				},
			},
		},
	}
	telegrafcfg := &config.Config{
		Inputs: []*models.RunningInput{
			{Config: &models.InputConfig{Name: "logs"}},
			{Config: &models.InputConfig{Name: "cpu"}},
		},
		Outputs: []*models.RunningOutput{
			{Config: &models.OutputConfig{Name: "cloudwatchlogs"}},
		},
	}

	SetComponents(otelcfg, telegrafcfg)
	assert.Equal(t, "cpu logs prometheus run_as_user", receivers)
	assert.Equal(t, "batch filter", processors)
	assert.Equal(t, "cloudwatch cloudwatchlogs", exporters)

	isRunningAsRoot = func() bool { return true }
	SetComponents(otelcfg, telegrafcfg)
	assert.Equal(t, "cpu logs prometheus", receivers)
	assert.Equal(t, "batch filter", processors)
	assert.Equal(t, "cloudwatch cloudwatchlogs", exporters)
	isRunningAsRoot = defaultIsRunningAsRoot
}

func TestGetAgentStats(t *testing.T) {
	stats := agentStats{
		CpuPercent:          aws.Float64(1.2),
		MemoryBytes:         aws.Uint64(123),
		FileDescriptorCount: aws.Int32(456),
		ThreadCount:         aws.Int32(789),
		LatencyMillis:       aws.Int64(1234),
		PayloadBytes:        aws.Int(5678),
		StatusCode:          aws.Int(200),
	}

	assert.Equal(t, "\"cpu\":1.2,\"mem\":123,\"fd\":456,\"th\":789,\"lat\":1234,\"load\":5678,\"code\":200", getAgentStats(stats))

	stats.CpuPercent = nil
	assert.Equal(t, "\"mem\":123,\"fd\":456,\"th\":789,\"lat\":1234,\"load\":5678,\"code\":200", getAgentStats(stats))

	stats.MemoryBytes = nil
	assert.Equal(t, "\"fd\":456,\"th\":789,\"lat\":1234,\"load\":5678,\"code\":200", getAgentStats(stats))

	stats.FileDescriptorCount = nil
	assert.Equal(t, "\"th\":789,\"lat\":1234,\"load\":5678,\"code\":200", getAgentStats(stats))

	stats.ThreadCount = nil
	assert.Equal(t, "\"lat\":1234,\"load\":5678,\"code\":200", getAgentStats(stats))

	stats.LatencyMillis = nil
	assert.Equal(t, "\"load\":5678,\"code\":200", getAgentStats(stats))

	stats.PayloadBytes = nil
	assert.Equal(t, "\"code\":200", getAgentStats(stats))

	stats.StatusCode = nil
	assert.Empty(t, getAgentStats(stats))

	stats.SharedConfigFallback = aws.Int(1)
	assert.Equal(t, "\"scfb\":1", getAgentStats(stats))
}

func TestGetStatusCode(t *testing.T) {
	assert.EqualValues(t, aws.Int(http.StatusOK), getStatusCode(nil))
	assert.Nil(t, getStatusCode(errors.New("")))
	assert.EqualValues(t, aws.Int(http.StatusBadRequest), getStatusCode(awserr.NewRequestFailure(awserr.New("", "", errors.New("")), http.StatusBadRequest, "")))
}

func TestGetUserAgent(t *testing.T) {
	assert.Equal(t, "TEST_FULL_VERSION", getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "TEST_RECEIVERS", "TEST_PROCESSORS", "TEST_EXPORTERS", false))

	expectedUserAgentRegex := `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`inputs:\(TEST_RECEIVERS\) processors:\(TEST_PROCESSORS\) outputs:\(TEST_EXPORTERS\)$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "TEST_RECEIVERS", "TEST_PROCESSORS", "TEST_EXPORTERS", true))

	expectedUserAgentRegex = `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`inputs:\(TEST_RECEIVERS\) processors:\(TEST_PROCESSORS\) outputs:\(TEST_EXPORTERS container_insights\)$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("/aws/containerinsights/test/performance", "TEST_FULL_VERSION", "TEST_RECEIVERS", "TEST_PROCESSORS", "TEST_EXPORTERS", true))
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("/aws/containerinsights/test/prometheus", "TEST_FULL_VERSION", "TEST_RECEIVERS", "TEST_PROCESSORS", "TEST_EXPORTERS", true))

	expectedUserAgentRegex = `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`inputs:\(TEST_RECEIVERS\) processors:\(TEST_PROCESSORS\)$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "TEST_RECEIVERS", "TEST_PROCESSORS", "", true))

	expectedUserAgentRegex = `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`inputs:\(TEST_RECEIVERS\) outputs:\(TEST_EXPORTERS\)$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "TEST_RECEIVERS", "", "TEST_EXPORTERS", true))

	expectedUserAgentRegex = `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`processors:\(TEST_PROCESSORS\) outputs:\(TEST_EXPORTERS\)$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "", "TEST_PROCESSORS", "TEST_EXPORTERS", true))

	expectedUserAgentRegex = `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "", "", "", true))

	t.Setenv(envconfig.CWAGENT_USER_AGENT, "TEST_USER_AGENT")
	assert.Equal(t, "TEST_USER_AGENT", getUserAgent("", "", "", "", "", false))
}

func TestGetVersion(t *testing.T) {
	expectedVersion := "Unknown"
	expectedFullVersion := fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		expectedVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
	assert.Equal(t, expectedVersion, Version())
	assert.Equal(t, expectedFullVersion, FullVersion())

	ex, err := os.Executable()
	require.NoError(t, err)

	expectedVersion = "TEST_VERSION"
	expectedFullVersion = fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		expectedVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
	vfp := filepath.Join(filepath.Dir(ex), versionFilename)
	err = os.WriteFile(vfp, []byte(expectedVersion), 0644)
	require.NoError(t, err)
	defer os.Remove(vfp)

	actualVersion := readVersionFile()
	assert.Equal(t, expectedVersion, actualVersion)
	assert.Equal(t, expectedFullVersion, getFullVersion(actualVersion))
}

func TestIsUsageDataEnabled(t *testing.T) {
	assert.True(t, getUsageDataEnabled())

	t.Setenv(envconfig.CWAGENT_USAGE_DATA, "TRUE")
	assert.True(t, getUsageDataEnabled())

	t.Setenv(envconfig.CWAGENT_USAGE_DATA, "INVALID")
	assert.True(t, getUsageDataEnabled())

	t.Setenv(envconfig.CWAGENT_USAGE_DATA, "FALSE")
	assert.False(t, getUsageDataEnabled())
}

func TestSharedConfigFallback(t *testing.T) {
	defer sharedConfigFallback.Store(false)
	assert.Nil(t, getSharedConfigFallback())
	RecordSharedConfigFallback()
	assert.Equal(t, 1, *(getSharedConfigFallback()))
	RecordSharedConfigFallback()
	RecordSharedConfigFallback()
	assert.Equal(t, 1, *(getSharedConfigFallback()))
}
