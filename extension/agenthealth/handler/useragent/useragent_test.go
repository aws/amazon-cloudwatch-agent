// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"sync"
	"testing"

	telegraf "github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"
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
	metricsType, _ := component.NewType("metrics")
	telegrafCPUType, _ := component.NewType(adapter.TelegrafPrefix + "cpu")
	prometheusType, _ := component.NewType("prometheus")
	batchType, _ := component.NewType("batch")
	filterType, _ := component.NewType("filter")
	cloudwatchType, _ := component.NewType("cloudwatch")
	otelCfg := &otelcol.Config{
		Service: service.Config{
			Pipelines: map[component.ID]*pipelines.PipelineConfig{
				component.NewID(metricsType): {
					Receivers: []component.ID{
						component.NewID(telegrafCPUType),
						component.NewID(prometheusType),
					},
					Processors: []component.ID{
						component.NewID(batchType),
						component.NewID(filterType),
					},
					Exporters: []component.ID{
						component.NewID(cloudwatchType),
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
	metricsType, _ := component.NewType("metrics")
	nopType, _ := component.NewType("nop")
	awsEMFType, _ := component.NewType("awsemf")
	otelCfg := &otelcol.Config{
		Service: service.Config{
			Pipelines: map[component.ID]*pipelines.PipelineConfig{
				component.NewID(metricsType): {
					Receivers: []component.ID{
						component.NewID(nopType),
					},
					Exporters: []component.ID{
						component.NewID(awsEMFType),
					},
				},
			},
		},
		Exporters: map[component.ID]component.Config{
			component.NewID(awsEMFType): &awsemfexporter.Config{Namespace: "AppSignals", LogGroupName: "/aws/appsignals/log/group"},
		},
	}
	ua := newUserAgent()
	ua.SetComponents(otelCfg, &telegraf.Config{})
	assert.Len(t, ua.inputs, 2)
	assert.Len(t, ua.processors, 0)
	assert.Len(t, ua.outputs, 2)

	assert.Equal(t, "inputs:(nop run_as_user)", ua.inputsStr.Load())
	assert.Equal(t, "", ua.processorsStr.Load())
	assert.Equal(t, "outputs:(application_signals awsemf)", ua.outputsStr.Load())
}

func TestJmx(t *testing.T) {
	jmx := "jmx"
	jmxOther := "jmxOther"
	nopType, _ := component.NewType("nop")
	jmxType, _ := component.NewType(jmx)
	pipelineType, _ := component.NewType("pipeline")
	pipelineTypeOther, _ := component.NewType("pipelineOther")
	pls := make(pipelines.Config)
	pls[component.NewID(pipelineType)] = &pipelines.PipelineConfig{
		Receivers: []component.ID{
			component.NewIDWithName(jmxType, jmx),
		},
		Exporters: []component.ID{
			component.NewID(nopType),
		},
	}
	pls[component.NewID(pipelineTypeOther)] = &pipelines.PipelineConfig{
		Receivers: []component.ID{
			component.NewIDWithName(jmxType, jmxOther),
		},
		Exporters: []component.ID{
			component.NewID(nopType),
		},
	}
	otelCfg := &otelcol.Config{
		Service: service.Config{
			Pipelines: pls,
		},
		Receivers: map[component.ID]component.Config{
			component.NewIDWithName(jmxType, jmx):      &jmxreceiver.Config{TargetSystem: "jvm,tomcat"},
			component.NewIDWithName(jmxType, jmxOther): &jmxreceiver.Config{TargetSystem: "jvm,kafka"},
		},
	}
	ua := newUserAgent()
	ua.SetComponents(otelCfg, &telegraf.Config{})
	assert.Len(t, ua.inputs, 5)
	assert.Len(t, ua.processors, 0)
	assert.Len(t, ua.outputs, 1)

	assert.Equal(t, "inputs:(jmx jmx-jvm jmx-kafka jmx-tomcat run_as_user)", ua.inputsStr.Load())
	assert.Equal(t, "", ua.processorsStr.Load())
	assert.Equal(t, "outputs:(nop)", ua.outputsStr.Load())
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
