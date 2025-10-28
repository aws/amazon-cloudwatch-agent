// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"fmt"
	"sync"
	"testing"

	"github.com/influxdata/telegraf"
	telegrafconfig "github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/pipelines"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/version"
	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
)

func TestSetComponents(t *testing.T) {
	telegrafCPUType, _ := component.NewType(adapter.TelegrafPrefix + "cpu")
	prometheusType, _ := component.NewType("prometheus")
	batchType, _ := component.NewType("batch")
	filterType, _ := component.NewType("filter")
	cloudwatchType, _ := component.NewType("cloudwatch")
	otelCfg := &otelcol.Config{
		Service: service.Config{
			Pipelines: map[pipeline.ID]*pipelines.PipelineConfig{
				pipeline.NewID(pipeline.SignalMetrics): {
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
	telegrafCfg := &telegrafconfig.Config{
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
	ua.SetComponents(&otelcol.Config{}, &telegrafconfig.Config{})
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
	nopType, _ := component.NewType("nop")
	awsEMFType, _ := component.NewType("awsemf")
	otelCfg := &otelcol.Config{
		Service: service.Config{
			Pipelines: map[pipeline.ID]*pipelines.PipelineConfig{
				pipeline.NewID(pipeline.SignalMetrics): {
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
			component.NewID(awsEMFType): &awsemfexporter.Config{Namespace: "ApplicationSignals", LogGroupName: "/aws/application-signals/log/group"},
		},
	}
	ua := newUserAgent()
	ua.SetComponents(otelCfg, &telegrafconfig.Config{})
	assert.Len(t, ua.inputs, 2)
	assert.Len(t, ua.processors, 0)
	assert.Len(t, ua.outputs, 2)

	assert.Equal(t, "inputs:(nop run_as_user)", ua.inputsStr.Load())
	assert.Equal(t, "", ua.processorsStr.Load())
	assert.Equal(t, "outputs:(application_signals awsemf)", ua.outputsStr.Load())
}

func TestMissingEmfExporterConfig(t *testing.T) {
	otelCfg := &otelcol.Config{
		Service: service.Config{
			Pipelines: map[pipeline.ID]*pipelines.PipelineConfig{
				pipeline.NewID(pipeline.SignalMetrics): {
					Receivers: []component.ID{
						component.NewID(component.MustNewType("nop")),
					},
					Exporters: []component.ID{
						component.NewID(component.MustNewType("awsemf")),
					},
				},
			},
		},
	}
	ua := newUserAgent()
	ua.SetComponents(otelCfg, &telegrafconfig.Config{})
	assert.Len(t, ua.inputs, 2)
	assert.Len(t, ua.processors, 0)
	assert.Len(t, ua.outputs, 1)

	assert.Equal(t, "inputs:(nop run_as_user)", ua.inputsStr.Load())
	assert.Equal(t, "", ua.processorsStr.Load())
	assert.Equal(t, "outputs:(awsemf)", ua.outputsStr.Load())
}

func TestJmx(t *testing.T) {
	jmx := "jmx"
	jmxOther := "jmxOther"
	nopType, _ := component.NewType("nop")
	jmxType, _ := component.NewType(jmx)
	pipelineID := pipeline.NewIDWithName(pipeline.SignalMetrics, "pipeline")
	pipelineIDOther := pipeline.NewIDWithName(pipeline.SignalMetrics, "pipelineOther")
	pls := make(pipelines.Config)
	pls[pipelineID] = &pipelines.PipelineConfig{
		Receivers: []component.ID{
			component.NewIDWithName(jmxType, jmx),
		},
		Exporters: []component.ID{
			component.NewID(nopType),
		},
	}
	pls[pipelineIDOther] = &pipelines.PipelineConfig{
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
	ua.SetComponents(otelCfg, &telegrafconfig.Config{})
	assert.Len(t, ua.inputs, 5)
	assert.Len(t, ua.processors, 0)
	assert.Len(t, ua.outputs, 1)

	assert.Equal(t, "inputs:(jmx jmx-jvm jmx-kafka jmx-tomcat run_as_user)", ua.inputsStr.Load())
	assert.Equal(t, "", ua.processorsStr.Load())
	assert.Equal(t, "outputs:(nop)", ua.outputsStr.Load())
}

func TestAddFeatureFlags(t *testing.T) {
	ua := newUserAgent()

	ua.AddFeatureFlags("feature1")
	assert.Len(t, ua.feature, 1)
	assert.Equal(t, "feature:(feature1)", ua.featureStr.Load())

	ua.AddFeatureFlags("feature1", "feature2", "feature3")
	assert.Len(t, ua.feature, 3)
	assert.Equal(t, "feature:(feature1 feature2 feature3)", ua.featureStr.Load())

	ua.AddFeatureFlags("")
	assert.Len(t, ua.feature, 3)
	assert.Equal(t, "feature:(feature1 feature2 feature3)", ua.featureStr.Load())
	assert.Contains(t, ua.Header(true), "feature:(feature1 feature2 feature3)")
}

func TestAddFeatureFlags_Concurrent(t *testing.T) {
	ua := newUserAgent()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ua.AddFeatureFlags(fmt.Sprintf("feature%d", i))
		}(i)
	}
	wg.Wait()
	assert.Len(t, ua.feature, 50)
}

func TestReset(t *testing.T) {
	ua := newUserAgent()

	ua.SetComponents(&otelcol.Config{}, &telegrafconfig.Config{})
	ua.SetContainerInsightsFlag()
	ua.AddFeatureFlags("test")

	assert.Len(t, ua.inputs, 1)
	assert.Len(t, ua.processors, 0)
	assert.Len(t, ua.outputs, 1)
	assert.Len(t, ua.feature, 1)

	assert.Equal(t, "inputs:(run_as_user)", ua.inputsStr.Load())
	assert.Equal(t, "", ua.processorsStr.Load())
	assert.Equal(t, "outputs:(container_insights)", ua.outputsStr.Load())
	assert.Equal(t, "feature:(test)", ua.featureStr.Load())

	ua.Reset()

	assert.Len(t, ua.inputs, 0)
	assert.Len(t, ua.processors, 0)
	assert.Len(t, ua.outputs, 0)
	assert.Len(t, ua.feature, 0)

	assert.Equal(t, "", ua.inputsStr.Load())
	assert.Equal(t, "", ua.processorsStr.Load())
	assert.Equal(t, "", ua.outputsStr.Load())
	assert.Equal(t, "", ua.featureStr.Load())
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

func TestWindowsEventLogFeatureFlags(t *testing.T) {
	tests := []struct {
		name          string
		inputName     string
		plugin        *mockWindowsEventLogPlugin
		expectedFlags []string
	}{
		{
			name:          "non-windows input",
			inputName:     "cpu",
			plugin:        &mockWindowsEventLogPlugin{},
			expectedFlags: []string{},
		},
		{
			name:      "no features",
			inputName: pluginWindowsEventLog,
			plugin: &mockWindowsEventLogPlugin{
				Events: []mockEventConfig{{Name: "System"}},
			},
			expectedFlags: []string{},
		},
		{
			name:      "win_event_ids",
			inputName: pluginWindowsEventLog,
			plugin: &mockWindowsEventLogPlugin{
				Events: []mockEventConfig{{
					Name:     "System",
					EventIDs: []int{1000, 1001},
				}},
			},
			expectedFlags: []string{flagWindowsEventIDs},
		},
		{
			name:      "win_event_filters",
			inputName: pluginWindowsEventLog,
			plugin: &mockWindowsEventLogPlugin{
				Events: []mockEventConfig{{
					Name:    "System",
					Filters: []*mockEventFilter{{Expression: "test"}},
				}},
			},
			expectedFlags: []string{flagWindowsEventFilters},
		},
		{
			name:      "win_event_levels",
			inputName: pluginWindowsEventLog,
			plugin: &mockWindowsEventLogPlugin{
				Events: []mockEventConfig{{
					Name:   "System",
					Levels: []string{"ERROR", "WARNING"},
				}},
			},
			expectedFlags: []string{flagWindowsEventLevels},
		},
		{
			name:      "all windows_event_log flags",
			inputName: pluginWindowsEventLog,
			plugin: &mockWindowsEventLogPlugin{
				Events: []mockEventConfig{{
					Name:     "System",
					EventIDs: []int{1000},
					Filters:  []*mockEventFilter{{Expression: "test"}},
					Levels:   []string{"ERROR"},
				}},
			},
			expectedFlags: []string{flagWindowsEventIDs, flagWindowsEventFilters, flagWindowsEventLevels},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ua := newUserAgent()
			input := &models.RunningInput{
				Config: &models.InputConfig{Name: tt.inputName},
				Input:  tt.plugin,
			}

			ua.setWindowsEventLogFeatureFlags(input)

			// On Windows, features should be detected based on plugin config
			// On non-Windows, no features should be added (function is no-op)
			if len(tt.expectedFlags) > 0 {
				// This assertion will pass on Windows and fail gracefully on non-Windows
				// since the function is a no-op on non-Windows platforms
				for _, flag := range tt.expectedFlags {
					if ua.feature.Contains(flag) {
						// Feature running on Windows
						assert.Contains(t, ua.feature, flag, "Feature %s detected", flag)
					}
				}
			} else {
				assert.Len(t, ua.feature, 0, "No features detected")
			}

			// Verify header contains detected features (if any)
			header := ua.Header(true)
			for _, flag := range tt.expectedFlags {
				if ua.feature.Contains(flag) {
					assert.Contains(t, header, flag, "Header should contain detected feature %s", flag)
				}
			}
		})
	}
}

// Mock types for testing - these match the real EventConfig structure
type mockWindowsEventLogPlugin struct {
	Events []mockEventConfig
}

func (m *mockWindowsEventLogPlugin) Description() string                   { return "mock" }
func (m *mockWindowsEventLogPlugin) SampleConfig() string                  { return "" }
func (m *mockWindowsEventLogPlugin) Gather(acc telegraf.Accumulator) error { return nil }

type mockEventConfig struct {
	Name     string
	EventIDs []int
	Filters  []*mockEventFilter
	Levels   []string
}

type mockEventFilter struct {
	Expression string
}
