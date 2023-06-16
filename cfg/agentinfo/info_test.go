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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestNew(t *testing.T) {
	ai := New("")
	expectedUserAgentRegex := `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$`

	assert.Regexp(t, regexp.MustCompile(expectedUserAgentRegex), ai.UserAgent())
}

func TestRecordOpData(t *testing.T) {
	var expectedStats agentStats
	ai := newAgentInfo("")

	stats := ai.StatsHeader()
	require.NoError(t, json.Unmarshal([]byte("{"+stats+"}"), &expectedStats))
	assert.Equal(t, expectedStats, ai.stats)

	ai.RecordOpData(100, 10, nil)
	stats = ai.StatsHeader()
	require.NoError(t, json.Unmarshal([]byte("{"+stats+"}"), &expectedStats))
	assert.Equal(t, expectedStats, ai.stats)
	assert.EqualValues(t, 100, *ai.stats.LatencyMillis)
	assert.EqualValues(t, 10, *ai.stats.PayloadBytes)
	assert.EqualValues(t, http.StatusOK, *ai.stats.StatusCode)

	ai.RecordOpData(200, 20, errors.New(""))
	stats = ai.StatsHeader()
	require.NoError(t, json.Unmarshal([]byte("{"+stats+"}"), &expectedStats))
	assert.EqualValues(t, 200, *ai.stats.LatencyMillis)
	assert.EqualValues(t, 20, *ai.stats.PayloadBytes)
	assert.Nil(t, ai.stats.StatusCode)

	ai.RecordOpData(300, 30, awserr.NewRequestFailure(awserr.New("", "", errors.New("")), 500, ""))
	stats = ai.StatsHeader()
	require.NoError(t, json.Unmarshal([]byte("{"+stats+"}"), &expectedStats))
	assert.EqualValues(t, 300, *ai.stats.LatencyMillis)
	assert.EqualValues(t, 30, *ai.stats.PayloadBytes)
	assert.EqualValues(t, 500, *ai.stats.StatusCode)
}

func TestSetComponents(t *testing.T) {
	telegrafcfg := &config.Config{
		Inputs: []*models.RunningInput{
			{Config: &models.InputConfig{Name: "logs"}},
			{Config: &models.InputConfig{Name: "cpu"}},
		},
		Outputs: []*models.RunningOutput{
			{Config: &models.OutputConfig{Name: "cloudwatch"}},
			{Config: &models.OutputConfig{Name: "cloudwatchlogs"}},
		},
	}

	SetComponents(telegrafcfg)
	assert.Equal(t, "cpu logs run_as_user", inputs)
	assert.Equal(t, "cloudwatch cloudwatchlogs", outputs)

	isRunningAsRoot = func() bool { return true }
	SetComponents(telegrafcfg)
	assert.Equal(t, "cpu logs", inputs)
	assert.Equal(t, "cloudwatch cloudwatchlogs", outputs)
	isRunningAsRoot = defaultIsRunningAsRoot
}

func TestGetAgentStats(t *testing.T) {
	latencyMillis := time.Duration(1234)
	stats := agentStats{
		CpuPercent:          aws.Float64(1.2),
		MemoryBytes:         aws.Uint64(123),
		FileDescriptorCount: aws.Int32(456),
		ThreadCount:         aws.Int32(789),
		LatencyMillis:       &latencyMillis,
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
}

func TestGetStatusCode(t *testing.T) {
	assert.EqualValues(t, aws.Int(http.StatusOK), getStatusCode(nil))
	assert.Nil(t, getStatusCode(errors.New("")))
	assert.EqualValues(t, aws.Int(http.StatusBadRequest), getStatusCode(awserr.NewRequestFailure(awserr.New("", "", errors.New("")), http.StatusBadRequest, "")))
}

func TestGetUserAgent(t *testing.T) {
	assert.Equal(t, "TEST_FULL_VERSION", getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "TEST_INPUTS", "TEST_OUTPUTS", false))

	expectedUserAgentRegex := `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`inputs:\(TEST_INPUTS\) outputs:\(TEST_OUTPUTS\)$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "TEST_INPUTS", "TEST_OUTPUTS", true))

	expectedUserAgentRegex = `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`inputs:\(TEST_INPUTS\) outputs:\(TEST_OUTPUTS container_insights\)$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("/aws/containerinsights/test/performance", "TEST_FULL_VERSION", "TEST_INPUTS", "TEST_OUTPUTS", true))
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("/aws/containerinsights/test/prometheus", "TEST_FULL_VERSION", "TEST_INPUTS", "TEST_OUTPUTS", true))

	expectedUserAgentRegex = `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`inputs:\(TEST_INPUTS\)$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "TEST_INPUTS", "", true))

	expectedUserAgentRegex = `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`outputs:\(TEST_OUTPUTS\)$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "", "TEST_OUTPUTS", true))

	expectedUserAgentRegex = `^TEST_FULL_VERSION ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$`
	assert.Regexp(t, expectedUserAgentRegex, getUserAgent("TEST_GROUP", "TEST_FULL_VERSION", "", "", true))

	t.Setenv(envconfig.CWAGENT_USER_AGENT, "TEST_USER_AGENT")
	assert.Equal(t, "TEST_USER_AGENT", getUserAgent("", "", "", "", false))
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
	assert.Equal(t, expectedFullVersion, getFullVersion(actualVersion, ""))
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
