// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentinfo

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/models"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/cfg/envconfig"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/receiver/adapter"
)

func TestVersionUseInjectedIfAvailable(t *testing.T) {
	injected := "VersionString"
	VersionStr = injected

	v := Version()
	if v != injected {
		t.Errorf("Wrong version returned %v, expecting %v", v, injected)
	}
}

func TestFallbackVersion(t *testing.T) {
	VersionStr = ""

	v := Version()
	if v != fallbackVersion {
		t.Errorf("Wrong version returned %v, expecting %v", v, fallbackVersion)
	}
}

func TestReadVersionFile(t *testing.T) {
	VersionStr = ""
	ex, err := os.Executable()
	if err != nil {
		t.Fatalf("cannot get the path for current executable binary: %v", err)
	}
	vfp := filepath.Join(filepath.Dir(ex), versionFilename)
	expectedVersion := "TEST_VERSION"

	if err = os.WriteFile(vfp, []byte(expectedVersion), 0644); err != nil {
		t.Fatalf("failed to write version file at %v: %v", vfp, err)
	}
	defer os.Remove(vfp)

	v := Version()
	if v != expectedVersion {
		t.Errorf("Wrong version returned %v, expecting %v", v, expectedVersion)
	}
}

func TestBuild(t *testing.T) {
	bstr := "SOME BUILD STR"
	BuildStr = bstr

	b := Build()
	if b != bstr {
		t.Errorf("wrong build string returne %v, expecting %v", b, bstr)
	}
}

func TestFullVersion(t *testing.T) {
	VersionStr = "VSTR"
	BuildStr = "BSTR"

	fv := FullVersion()
	fvp := strings.Split(fv, " ")
	if fvp[0] != "CWAgent/VSTR" || fvp[4] != "BSTR" {
		t.Errorf("wrong FullVersion returned '%v' VSTR and BSTR not found", fv)
	}
}

func TestPlugins(t *testing.T) {
	receivers = []string{"a", "b", "c"}
	processors = []string{"i", "j", "k"}
	exporters = []string{"x", "y", "z"}

	isRunningAsRoot = func() bool { return true }
	plugins := Plugins("")
	expected := "inputs:(a b c) processors:(i j k) outputs:(x y z)"
	if plugins != expected {
		t.Errorf("wrong plugins string constructed '%v', expecting '%v'", plugins, expected)
	}

	plugins = Plugins("/aws/ecs/containerinsights/ecs-cluster-name/performance")
	expected = "inputs:(a b c) processors:(i j k) outputs:(x y z container_insights)"
	if plugins != expected {
		t.Errorf("wrong plugins string constructed '%v', expecting '%v'", plugins, expected)
	}

	isRunningAsRoot = func() bool { return false }
	plugins = Plugins("")
	expected = "inputs:(a b c run_as_user) processors:(i j k) outputs:(x y z)"
	if plugins != expected {
		t.Errorf("wrong plugins string constructed '%v', expecting '%v'", plugins, expected)
	}
}

func TestSetPlugins(t *testing.T) {
	otelcfg := &otelcol.Config{
		Service: service.ConfigService{
			Pipelines: map[component.ID]*service.ConfigServicePipeline{
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
	SetPlugins(otelcfg, telegrafcfg)
	assert.Equal(t, []string{"cpu", "logs", "prometheus"}, receivers)
	assert.Equal(t, []string{"processor"}, processors)
	assert.Equal(t, []string{"cloudwatch", "cloudwatchlogs"}, exporters)
}

func TestSetPluginsTelegrafOnly(t *testing.T) {
	otelcfg := &otelcol.Config{}
	telegrafcfg := &config.Config{
		Inputs: []*models.RunningInput{
			{Config: &models.InputConfig{Name: "logs"}},
		},
		Outputs: []*models.RunningOutput{
			{Config: &models.OutputConfig{Name: "cloudwatchlogs"}},
		},
	}
	SetPlugins(otelcfg, telegrafcfg)
	assert.Equal(t, []string{"logs"}, receivers)
	assert.Equal(t, []string{"cloudwatchlogs"}, exporters)
}

func TestUserAgent(t *testing.T) {
	VersionStr = "VSTR"
	BuildStr = "BSTR"
	receivers = []string{"a", "b", "c"}
	processors = []string{"i", "j", "k"}
	exporters = []string{"x", "y", "z"}

	isRunningAsRoot = func() bool { return true }

	tests := []struct {
		name         string
		logGroupName string
		expected     string
		errorMessage string
	}{
		{
			"non container insights",
			"test-group",
			fmt.Sprintf("CWAgent/VSTR (%v; %v; %v) BSTR inputs:(a b c) processors:(i j k) outputs:(x y z)", runtime.Version(), runtime.GOOS, runtime.GOARCH),
			"wrong UserAgent string constructed",
		},
		{
			"container insights EKS",
			"/aws/containerinsights/eks-cluster-name/performance",
			fmt.Sprintf("CWAgent/VSTR (%v; %v; %v) BSTR inputs:(a b c) processors:(i j k) outputs:(x y z container_insights)", runtime.Version(), runtime.GOOS, runtime.GOARCH),
			"\"container_insights\" flag shoould be in the outputs plugin list in container insights mode",
		},
		{
			"container insights ECS",
			"/aws/ecs/containerinsights/ecs-cluster-name/performance",
			fmt.Sprintf("CWAgent/VSTR (%v; %v; %v) BSTR inputs:(a b c) processors:(i j k) outputs:(x y z container_insights)", runtime.Version(), runtime.GOOS, runtime.GOARCH),
			"\"container_insights\" flag shoould be in the outputs plugin list in container insights mode",
		},
		{
			"container insights prometheus",
			"/aws/containerinsights/cluster-name/prometheus",
			fmt.Sprintf("CWAgent/VSTR (%v; %v; %v) BSTR inputs:(a b c) processors:(i j k) outputs:(x y z container_insights)", runtime.Version(), runtime.GOOS, runtime.GOARCH),
			"\"container_insights\" flag shoould be in the outputs plugin list in container insights mode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, UserAgent(tc.logGroupName), tc.errorMessage)
		})
	}
}

func TestUserAgentEnvOverride(t *testing.T) {
	t.Setenv(envconfig.CWAGENT_USER_AGENT, "CUSTOM CWAGENT USER AGENT")
	expected := "CUSTOM CWAGENT USER AGENT"

	ua := UserAgent("TestUserAgentEnvOverride")
	if ua != expected {
		t.Errorf("UserAgent should use value configured in environment variable CWAGENT_USER_AGENT, but '%v' found, expecting '%v'", ua, expected)
	}
}
