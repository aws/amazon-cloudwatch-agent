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

	"github.com/aws/aws-sdk-go/aws/awserr"
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

func TestNew(t *testing.T) {
	ai := New()
	expectedFullVersion := fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		unknownVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
	expectedUserAgentRegex := `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$`

	assert.Equal(t, unknownVersion, ai.Version())
	assert.Equal(t, expectedFullVersion, ai.FullVersion())
	assert.Regexp(t, regexp.MustCompile(expectedUserAgentRegex), ai.UserAgent())
}

func TestNewVersionFromFile(t *testing.T) {
	ex, err := os.Executable()
	require.NoError(t, err)

	expectedVersion := "TEST_VERSION"
	expectedFullVersion := fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		expectedVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
	expectedUserAgentRegex := `^CWAgent/TEST_VERSION \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$`

	vfp := filepath.Join(filepath.Dir(ex), versionFilename)
	err = os.WriteFile(vfp, []byte(expectedVersion), 0644)
	require.NoError(t, err)
	defer os.Remove(vfp)

	ai := New()
	assert.Equal(t, expectedVersion, ai.Version())
	assert.Equal(t, expectedFullVersion, ai.FullVersion())
	assert.Regexp(t, regexp.MustCompile(expectedUserAgentRegex), ai.UserAgent())
}

func TestNewUUIDFromFile(t *testing.T) {
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

func TestNewUUIDFromFileInvalid(t *testing.T) {
	ex, err := os.Executable()
	require.NoError(t, err)

	idFilePath := filepath.Join(filepath.Dir(ex), idFilename)
	err = os.WriteFile(idFilePath, []byte("INVALID"), 0644)
	require.NoError(t, err)
	defer os.Remove(idFilePath)

	ai := newAgentInfo()
	assert.NotEqual(t, "INVALID", ai.id)
}

func TestNewUserAgentEnvOverride(t *testing.T) {
	expectedUserAgent := "TEST USER AGENT"
	t.Setenv(envconfig.CWAGENT_USER_AGENT, expectedUserAgent)
	ai := newAgentInfo()

	assert.Equal(t, expectedUserAgent, ai.UserAgent())
	assert.True(t, ai.isTelemetryDisabled())
}

func TestNewUsageDataEnvOverride(t *testing.T) {
	t.Setenv(envconfig.CWAGENT_USAGE_DATA, "FALSE")
	ai := newAgentInfo()
	assert.True(t, ai.isTelemetryDisabled())

	t.Setenv(envconfig.CWAGENT_USAGE_DATA, "TRUE")
	ai = newAgentInfo()
	assert.False(t, ai.isTelemetryDisabled())
}

func TestOp(t *testing.T) {
	ai := New()
	ai.RecordOpData(100, 10, nil)

	expectedRegexp := `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`\(Receivers: -; Processors: -; Exporters: -; ` +
		`Telemetry: ([0-9]{1,3}.[0-9]|-) ([0-9]*|-) ([0-9]*|-) ([0-9]*|-) 100 10 200\)$`
	assert.Regexp(t, regexp.MustCompile(expectedRegexp), ai.UserAgent())
}

func TestOpConsecutive(t *testing.T) {
	ai := New()

	ai.RecordOpData(100, 10, nil)
	expectedRegexp := `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`\(Receivers: -; Processors: -; Exporters: -; ` +
		`Telemetry: ([0-9]{1,3}.[0-9]|-) ([0-9]*|-) ([0-9]*|-) ([0-9]*|-) 100 10 200\)$`
	assert.Regexp(t, regexp.MustCompile(expectedRegexp), ai.UserAgent())

	ai.RecordOpData(100, 11, nil)
	expectedRegexp = `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`\(Receivers: -; Processors: -; Exporters: -; ` +
		`Telemetry: ([0-9]{1,3}.[0-9]|-) ([0-9]*|-) ([0-9]*|-) ([0-9]*|-) 100 11 200\)$`
	assert.Regexp(t, regexp.MustCompile(expectedRegexp), ai.UserAgent())
}

func TestOpErr(t *testing.T) {
	ai := New()
	ai.RecordOpData(100, 10, errors.New(""))

	expectedRegexp := `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`\(Receivers: -; Processors: -; Exporters: -; ` +
		`Telemetry: ([0-9]{1,3}.[0-9]|-) ([0-9]*|-) ([0-9]*|-) ([0-9]*|-) 100 10 -\)$`
	assert.Regexp(t, regexp.MustCompile(expectedRegexp), ai.UserAgent())
}

func TestOpRequestFailure(t *testing.T) {
	ai := New()
	reqErr := awserr.NewRequestFailure(awserr.New("", "", errors.New("")), 500, "")
	ai.RecordOpData(100, 10, reqErr)

	expectedRegexp := `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12} ` +
		`\(Receivers: -; Processors: -; Exporters: -; ` +
		`Telemetry: ([0-9]{1,3}.[0-9]|-) ([0-9]*|-) ([0-9]*|-) ([0-9]*|-) 100 10 500\)$`
	assert.Regexp(t, regexp.MustCompile(expectedRegexp), ai.UserAgent())
}

func TestOpWithNilProc(t *testing.T) {
	ai := newAgentInfo()
	ai.proc = nil
	assert.True(t, ai.isTelemetryDisabled())

	ai.RecordOpData(100, 0, nil)
	expectedUserAgentRegex := `^CWAgent/Unknown \(.*\) ` +
		`ID/[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$`
	assert.Regexp(t, regexp.MustCompile(expectedUserAgentRegex), ai.UserAgent())
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
	SetComponents(otelcfg, telegrafcfg)
	assert.Equal(t, "Receivers: cpu logs prometheus; Processors: processor; Exporters: cloudwatch cloudwatchlogs", components)
}

func TestTelemetryToStr(t *testing.T) {
	assert.Equal(t, "-", telemetryToStr(0.0, errors.New("")))
	assert.Equal(t, "1.9", telemetryToStr(1.901, nil))
	assert.Equal(t, "10", telemetryToStr(uint64(10), nil))
}
