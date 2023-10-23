// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"sync"
	"testing"

	telegraf "github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/pipelines"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/version"
	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
)

func TestSetComponents(t *testing.T) {
	otelCfg := &otelcol.Config{
		Service: service.Config{
			Pipelines: map[component.ID]*pipelines.PipelineConfig{
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
	telegrafCfg := &telegraf.Config{
		Inputs: []*models.RunningInput{
			{Config: &models.InputConfig{Name: "logs"}},
			{Config: &models.InputConfig{Name: "cpu"}},
		},
		Outputs: []*models.RunningOutput{
			{Config: &models.OutputConfig{Name: "cloudwatchlogs"}},
		},
	}

	ua := newUserAgent()
	ua.isRoot = true
	ua.SetComponents(otelCfg, telegrafCfg)
	assert.Len(t, ua.inputs, 3)
	assert.Len(t, ua.processors, 2)
	assert.Len(t, ua.outputs, 2)

	assert.Equal(t, "inputs:(cpu logs prometheus)", ua.inputsStr.Load())
	assert.Equal(t, "processors:(batch filter)", ua.processorsStr.Load())
	assert.Equal(t, "outputs:(cloudwatch cloudwatchlogs)", ua.outputsStr.Load())
	assert.Contains(t, ua.Header(true), "inputs:(cpu logs prometheus) processors:(batch filter) outputs:(cloudwatch cloudwatchlogs)")

	ua.isRoot = false
	ua.SetComponents(otelCfg, telegrafCfg)
	assert.Len(t, ua.inputs, 4)
	assert.Equal(t, "inputs:(cpu logs prometheus run_as_user)", ua.inputsStr.Load())
}

func TestSetComponentsEmpty(t *testing.T) {
	ua := newUserAgent()
	ua.SetComponents(&otelcol.Config{}, &telegraf.Config{})
	assert.Len(t, ua.inputs, 1)
	assert.Len(t, ua.processors, 0)
	assert.Len(t, ua.outputs, 0)

	assert.Equal(t, "inputs:(run_as_user)", ua.inputsStr.Load())
	assert.Equal(t, "", ua.processorsStr.Load())
	assert.Equal(t, "", ua.outputsStr.Load())
}

func TestContainerInsightsFlag(t *testing.T) {
	ua := newUserAgent()
	ua.outputs.Add("TEST_EXPORTER")
	ua.SetContainerInsightsFlag()
	assert.Equal(t, "outputs:(TEST_EXPORTER container_insights)", ua.outputsStr.Load())
	// do not rebuild output string if flag already set
	ua.outputs.Add("flag_already_set")
	ua.SetContainerInsightsFlag()
	assert.Equal(t, "outputs:(TEST_EXPORTER container_insights)", ua.outputsStr.Load())
}

func TestAlternateUserAgent(t *testing.T) {
	t.Setenv(envconfig.CWAGENT_USER_AGENT, "TEST_AGENT")
	ua := newUserAgent()
	assert.Equal(t, "TEST_AGENT", ua.Header(false))
	t.Setenv(envconfig.CWAGENT_USER_AGENT, "")
	assert.Equal(t, version.Full(), ua.Header(false))
}

func TestEmf(t *testing.T) {
	otelCfg := &otelcol.Config{
		Service: service.Config{
			Pipelines: map[component.ID]*pipelines.PipelineConfig{
				component.NewID("metrics"): {
					Receivers: []component.ID{
						component.NewID("nop"),
					},
					Exporters: []component.ID{
						component.NewID("awsemf"),
					},
				},
			},
		},
		Exporters: map[component.ID]component.Config{
			component.NewID("awsemf"): &awsemfexporter.Config{Namespace: "AWS/APM", LogGroupName: "/aws/apm/log/group"},
		},
	}
	ua := newUserAgent()
	ua.SetComponents(otelCfg, &telegraf.Config{})
	assert.Len(t, ua.inputs, 2)
	assert.Len(t, ua.processors, 0)
	assert.Len(t, ua.outputs, 2)

	assert.Equal(t, "inputs:(nop run_as_user)", ua.inputsStr.Load())
	assert.Equal(t, "", ua.processorsStr.Load())
	assert.Equal(t, "outputs:(awsemf pulse)", ua.outputsStr.Load())
}

func TestSingleton(t *testing.T) {
	assert.Equal(t, Get().(*userAgent).id, Get().(*userAgent).id)
}

func TestListen(t *testing.T) {
	var wg sync.WaitGroup
	ua := newUserAgent()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		ua.Listen(wg.Done)
	}
	ua.SetContainerInsightsFlag()
	wg.Wait()
}
