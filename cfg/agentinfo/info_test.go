// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentinfo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/cfg/envconfig"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/receiver/adapter"
)

func TestGet(t *testing.T) {
	ai := Get()
	require.NotNil(t, ai)
	ai.Start()
	ai.Shutdown()
}

func TestVersionUnknown(t *testing.T) {
	expectedFullVersion := fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		unknownVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
	ai := newAgentInfo()
	assert.Equal(t, unknownVersion, ai.Version())
	assert.Equal(t, expectedFullVersion, ai.FullVersion())
}

func TestVersionFile(t *testing.T) {
	ex, err := os.Executable()
	require.NoError(t, err)

	vfp := filepath.Join(filepath.Dir(ex), versionFilename)
	expectedVersion := "TEST_VERSION"
	expectedFullVersion := fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		expectedVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)

	err = os.WriteFile(vfp, []byte(expectedVersion), 0644)
	require.NoError(t, err)
	defer os.Remove(vfp)

	ai := newAgentInfo()
	assert.Equal(t, expectedVersion, ai.Version())
	assert.Equal(t, expectedFullVersion, ai.FullVersion())
}

func TestCreateTelemetry(t *testing.T) {
	ai := newAgentInfo()
	expectedRegexp := `^Telemetry: ([0-9]{1,3}.[0-9]|-) ([0-9]*|-) ([0-9]*|-) ([0-9]*|-)$`
	assert.Regexp(t, regexp.MustCompile(expectedRegexp), ai.createTelemetry())
}

func TestTelemetryToStr(t *testing.T) {
	assert.Equal(t, "-", telemetryToStr(0.0, errors.New("")))
	assert.Equal(t, "1.9", telemetryToStr(1.901, nil))
	assert.Equal(t, "10", telemetryToStr(uint64(10), nil))
}

func TestSetPlugins(t *testing.T) {
	ai := newAgentInfo()
	otelcfg := &otelcol.Config{
		Service: service.Config{
			Pipelines: map[component.ID]*service.PipelineConfig{
				component.NewID("metrics"): {
					Receivers: []component.ID{
						component.NewID(adapter.TelegrafPrefix + "cpu"),
						component.NewID("prometheus"),
					},
					Processors: []component.ID{
						component.NewID("processor"),
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
	ai.SetPlugins(otelcfg, telegrafcfg)
	assert.Equal(t, "Inputs:cpu logs prometheus; Processors:processor; Outputs:cloudwatch cloudwatchlogs", ai.plugins)
}

func TestCreateUserAgent(t *testing.T) {
	ai := newAgentInfo()
	ai.proc = nil
	ai.plugins = "Inputs: a b; Processors: c d; Outputs: e f"

	expectedRegexp := `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`\(Inputs: a b; Processors: c d; Outputs: e f\)$`
	assert.Regexp(t, regexp.MustCompile(expectedRegexp), ai.createUserAgent())
}

func TestCreateUserAgentWithTelemetry(t *testing.T) {
	ai := newAgentInfo()
	ai.telemetry = ai.createTelemetry()
	ai.plugins = "Inputs: a b; Processors: c d; Outputs: e f"

	expectedRegexp := `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`\(Inputs: a b; Processors: c d; Outputs: e f; Telemetry: .*\)$`
	assert.Regexp(t, regexp.MustCompile(expectedRegexp), ai.createUserAgent())
}

func TestCreateUserAgentEnvOverride(t *testing.T) {
	expectedUserAgent := "TEST USER AGENT"
	t.Setenv(envconfig.CWAGENT_USER_AGENT, expectedUserAgent)
	ai := newAgentInfo()

	assert.Equal(t, expectedUserAgent, ai.UserAgent())
}

func TestReadOrWriteUUIDFile(t *testing.T) {
	ex, err := os.Executable()
	require.NoError(t, err)

	idFilePath := filepath.Join(filepath.Dir(ex), idFilename)
	expectedID := "123e4567-e89b-12d3-a456-426614174000"

	err = os.WriteFile(idFilePath, []byte(expectedID), 0644)
	require.NoError(t, err)
	defer os.Remove(idFilePath)

	ai := newAgentInfo()
	assert.Equal(t, expectedID, ai.id)
}
