# Extension Patterns Handoff: file_storage for journald cursor persistence

## Key Findings

1. **filestorage.NewFactory() is already registered** in `service/defaultcomponents/components.go` — no need to add it
2. **Extension translator pattern**: Create a struct implementing `common.ComponentTranslator` with `ID()` and `Translate()` methods
3. **Pipeline wiring**: Extensions are added via `translators.Extensions.Set(...)` in the pipeline translator
4. **Receiver storage**: `journaldreceiver.JournaldConfig` has `BaseConfig.StorageID` field (currently nil, confirmed in test)
5. **Import path**: `github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage` (in go.mod)

## Implementation Plan

1. Create `translator/translate/otel/extension/filestorage/translator.go` — new extension translator
2. Create `translator/translate/otel/extension/filestorage/translator_test.go` — tests
3. Modify `translator/translate/otel/pipeline/journald/translator.go` — wire file_storage extension
4. Modify `translator/translate/otel/receiver/journald/translator.go` — set StorageID on receiver config

---

## File 1: translator/translate/otel/extension/agenthealth/translator.go
**Purpose**: Reference pattern for creating a new extension translator

```go
// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"maps"
	"slices"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/metadata"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	translateagent "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	OperationPutMetricData    = "PutMetricData"
	OperationPutLogEvents     = "PutLogEvents"
	OperationPutTraceSegments = "PutTraceSegments"

	usageDataKey     = "usage_data"
	usageMetadataKey = "usage_metadata"
)

var (
	MetricsID    = component.NewIDWithName(agenthealth.TypeStr, pipeline.SignalMetrics.String())
	LogsID       = component.NewIDWithName(agenthealth.TypeStr, pipeline.SignalLogs.String())
	TracesID     = component.NewIDWithName(agenthealth.TypeStr, pipeline.SignalTraces.String())
	StatusCodeID = component.NewIDWithName(agenthealth.TypeStr, "statuscode")
)

type Name string

var (
	MetricsName    = Name(pipeline.SignalMetrics.String())
	LogsName       = Name(pipeline.SignalLogs.String())
	TracesName     = Name(pipeline.SignalTraces.String())
	StatusCodeName = Name("statuscode")
)

type translator struct {
	name                string
	operations          []string
	isUsageDataEnabled  bool
	factory             extension.Factory
	isStatusCodeEnabled bool
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslatorWithStatusCode(name Name, operations []string, isStatusCodeEnabled bool) common.ComponentTranslator {
	return &translator{
		name:                string(name),
		operations:          operations,
		factory:             agenthealth.NewFactory(),
		isUsageDataEnabled:  envconfig.IsUsageDataEnabled(),
		isStatusCodeEnabled: isStatusCodeEnabled,
	}
}

func NewTranslator(name Name, operations []string) common.ComponentTranslator {
	return &translator{
		name:               string(name),
		operations:         operations,
		factory:            agenthealth.NewFactory(),
		isUsageDataEnabled: envconfig.IsUsageDataEnabled(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an extension configuration.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*agenthealth.Config)
	cfg.IsUsageDataEnabled = t.isUsageDataEnabled
	if usageData, ok := common.GetBool(conf, common.ConfigKey(common.AgentKey, usageDataKey)); ok {
		cfg.IsUsageDataEnabled = cfg.IsUsageDataEnabled && usageData
	}
	usageMetadata := common.GetArray[map[string]any](conf, common.ConfigKey(common.AgentKey, usageMetadataKey))
	usageMetadataSet := collections.NewSet[string]()
	for _, umd := range usageMetadata {
		for k, v := range umd {
			valueStr, ok := v.(string)
			if !ok {
				continue
			}
			if md := metadata.Build(k, valueStr); metadata.IsSupported(md) {
				usageMetadataSet.Add(md)
			}
		}
	}
	if len(usageMetadataSet) > 0 {
		cfg.UsageMetadata = slices.Sorted(maps.Keys(usageMetadataSet))
	}
	cfg.IsStatusCodeEnabled = t.isStatusCodeEnabled
	cfg.Stats = &agent.StatsConfig{
		Operations: t.operations,
		UsageFlags: map[agent.Flag]any{
			agent.FlagMode:       context.CurrentContext().ShortMode(),
			agent.FlagRegionType: translateagent.Global_Config.RegionType,
		},
	}
	return cfg, nil
}
```

---

## File 2: translator/translate/otel/extension/agenthealth/translator_test.go
**Purpose**: Test pattern for extension translators

```go
// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/metadata"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	translateagent "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

func TestTranslate(t *testing.T) {
	context.CurrentContext().SetMode(config.ModeEC2)
	translateagent.Global_Config.RegionType = config.RegionTypeNotFound
	operations := []string{OperationPutLogEvents}
	usageFlags := map[agent.Flag]any{
		agent.FlagMode:       config.ShortModeEC2,
		agent.FlagRegionType: config.RegionTypeNotFound,
	}
	testCases := map[string]struct {
		input          map[string]any
		isEnvUsageData bool
		want           *agenthealth.Config
	}{
		"WithUsageData/NotInConfig": {
			input:          map[string]any{"agent": map[string]any{}},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
			},
		},
		"WithUsageData/FalseInConfig": {
			input:          map[string]any{"agent": map[string]any{"usage_data": false}},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: false,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
			},
		},
		"WithUsageData/FalseInEnv": {
			input:          map[string]any{"agent": map[string]any{"usage_data": true}},
			isEnvUsageData: false,
			want: &agenthealth.Config{
				IsUsageDataEnabled: false,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
			},
		},
		"WithUsageData/BothTrue": {
			input:          map[string]any{"agent": map[string]any{"usage_data": true}},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
			},
		},
		"WithUsageMetadata/OnlyUnsupported": {
			input: map[string]any{
				"agent": map[string]any{
					"usage_data": true,
					"usage_metadata": []any{map[string]any{
						"unsupported_key": "unsupported_value",
					},
					},
				},
			},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
			},
		},
		"WithUsageMetadata/Mixed": {
			input: map[string]any{
				"agent": map[string]any{
					"usage_data": true,
					"usage_metadata": []any{
						map[string]any{
							"ObservabilitySolution": "jvm",
							"test":                  "value",
						},
						map[string]any{
							"unsupported_key": "unsupported_value",
						},
					},
				},
			},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
				UsageMetadata: []metadata.Metadata{"obs_jvm"},
			},
		},
		"WithUsageMetadata/Supported": {
			input:          testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
				UsageMetadata: []metadata.Metadata{"obs_jvm"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator(LogsName, operations).(*translator)
			assert.Equal(t, "agenthealth/logs", tt.ID().String())
			tt.isUsageDataEnabled = testCase.isEnvUsageData
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
```

---

## File 3: translator/translate/otel/pipeline/journald/translator.go
**Purpose**: Shows how extensions are wired into the pipeline via `translators.Extensions.Set(...)`

```go
// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/journald"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/journaldfilter"
	journaldreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/journald"
)

type translator struct {
	name string
}

var _ common.PipelineTranslator = (*translator)(nil)

const (
	pipelineName = "journald"
)

func NewTranslator() common.PipelineTranslator {
	return &translator{name: pipelineName}
}

func NewTranslators(conf *confmap.Conf) common.TranslatorMap[*common.ComponentTranslators, pipeline.ID] {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()

	journaldKey := common.ConfigKey(common.LogsKey, "logs_collected", "journald")
	if conf != nil && conf.IsSet(journaldKey) {
		// Create a single pipeline translator that handles all collect_list entries
		translators.Set(NewTranslator())
	}

	return translators
}



func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalLogs, t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	journaldKey := common.ConfigKey(common.LogsKey, "logs_collected", "journald")
	if conf == nil || !conf.IsSet(journaldKey) {
		return nil, &common.MissingKeyError{ID: component.NewID(component.MustNewType("journald")), JsonKey: journaldKey}
	}

	// Get the journald configuration
	journaldConf, err := conf.Sub(journaldKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get journald configuration: %w", err)
	}
	if journaldConf == nil {
		return nil, fmt.Errorf("journald configuration not found")
	}

	collectList := journaldConf.Get("collect_list")
	if collectList == nil {
		return nil, fmt.Errorf("collect_list not found in journald configuration")
	}

	collectListSlice, ok := collectList.([]interface{})
	if !ok || len(collectListSlice) == 0 {
		return nil, fmt.Errorf("collect_list is empty or invalid")
	}

	translators := common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	// Process each collect_list entry to create separate components
	for i, collectEntry := range collectListSlice {
		entryConfig, ok := collectEntry.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid collect_list entry at index %d", i)
		}

		// Create unique suffix for multiple entries
		suffix := ""
		if len(collectListSlice) > 1 {
			suffix = fmt.Sprintf("_%d", i)
		}

		// Add journald receiver for this entry
		receiverName := "journald" + suffix
		units, _ := entryConfig["units"].([]interface{})
		var unitStrings []string
		for _, unit := range units {
			if unitStr, ok := unit.(string); ok {
				unitStrings = append(unitStrings, unitStr)
			}
		}
		translators.Receivers.Set(journaldreceiver.NewTranslatorWithUnits(receiverName, unitStrings))

		// Add filter processor if filters are specified
		if filters, ok := entryConfig["filters"].([]interface{}); ok && len(filters) > 0 {
			var filterConfigs []journaldfilter.FilterConfig
			for _, filter := range filters {
				if filterMap, ok := filter.(map[string]interface{}); ok {
					filterType, _ := filterMap["type"].(string)
					expression, _ := filterMap["expression"].(string)
					if filterType != "" && expression != "" {
						filterConfigs = append(filterConfigs, journaldfilter.FilterConfig{
							Type:       filterType,
							Expression: expression,
						})
					}
				}
			}
			if len(filterConfigs) > 0 {
				filterName := "journald" + suffix
				translators.Processors.Set(journaldfilter.NewTranslatorWithFilters(filterName, filterConfigs))
			}
		}

		// Add batch processor for performance
		batchName := "journald" + suffix
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(batchName, common.LogsKey))

		// Add journald exporter with specific config for this collect_list entry
		exporterName := "journald" + suffix
		journaldExporter := journald.NewTranslatorWithConfig(exporterName, entryConfig)
		translators.Exporters.Set(journaldExporter)
	}

	// Add health extension
	translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}))
	translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true))

	return &translators, nil
}
```

---

## File 4: translator/translate/otel/pipeline/journald/translator_test.go
**Purpose**: Test patterns for pipeline translator

```go
// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		input   map[string]interface{}
		wantErr bool
	}{
		"WithValidConfig": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"log_group_name":    "system-logs",
									"log_stream_name":   "{instance_id}",
									"retention_in_days": 7,
									"units":             []interface{}{"systemd", "kernel", "sshd"},
									"filters": []interface{}{
										map[string]interface{}{
											"type":       "exclude",
											"expression": ".*debug.*",
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		"WithMissingConfig": {
			input:   map[string]interface{}{},
			wantErr: true,
		},
		"WithEmptyCollectList": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"collect_list": []interface{}{},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			translator := NewTranslator()

			// Verify ID
			expectedID := pipeline.NewIDWithName(pipeline.SignalLogs, pipelineName)
			assert.Equal(t, expectedID, translator.ID())

			got, err := translator.Translate(conf)

			if testCase.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)

			// Verify components are created
			assert.True(t, got.Receivers.Len() > 0, "Should have at least one receiver")
			assert.True(t, got.Processors.Len() > 0, "Should have at least one processor")
			assert.True(t, got.Exporters.Len() > 0, "Should have at least one exporter")
			assert.True(t, got.Extensions.Len() > 0, "Should have at least one extension")
		})
	}
}

func TestNewTranslators(t *testing.T) {
	testCases := map[string]struct {
		input       map[string]interface{}
		expectCount int
	}{
		"WithJournaldConfig": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"log_group_name": "test-logs",
								},
							},
						},
					},
				},
			},
			expectCount: 1,
		},
		"WithoutJournaldConfig": {
			input:       map[string]interface{}{},
			expectCount: 0,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			translators := NewTranslators(conf)
			assert.Equal(t, testCase.expectCount, translators.Len())
		})
	}
}```

---

## File 5: translator/translate/otel/receiver/journald/translator.go
**Purpose**: Receiver translator where StorageID needs to be set for cursor persistence

```go
// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory receiver.Factory
}

type translatorWithUnits struct {
	*translator
	units []string
}

var _ common.ComponentTranslator = (*translator)(nil)
var _ common.ComponentTranslator = (*translatorWithUnits)(nil)

func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{
		name:    name,
		factory: journaldreceiver.NewFactory(),
	}
}

func NewTranslatorWithUnits(name string, units []string) common.ComponentTranslator {
	return &translatorWithUnits{
		translator: &translator{
			name:    name,
			factory: journaldreceiver.NewFactory(),
		},
		units: units,
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	journaldKey := common.ConfigKey(common.LogsKey, "logs_collected", "journald")
	if conf == nil || !conf.IsSet(journaldKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: journaldKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)

	// Get the journald configuration from logs.logs_collected.journald
	journaldConf, err := conf.Sub(journaldKey)
	if err != nil {
		return nil, fmt.Errorf("error getting journald configuration: %w", err)
	}
	if journaldConf == nil {
		return nil, fmt.Errorf("journald configuration not found")
	}

	collectList := journaldConf.Get("collect_list")
	if collectList == nil {
		return nil, fmt.Errorf("collect_list not found in journald configuration")
	}

	// For now, we'll use the first collect_list entry to configure the receiver
	// In a full implementation, we'd need to create multiple receivers or handle multiple configs
	collectListSlice, ok := collectList.([]interface{})
	if !ok || len(collectListSlice) == 0 {
		return nil, fmt.Errorf("collect_list is empty or invalid")
	}

	firstConfig, ok := collectListSlice[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid collect_list entry")
	}

	// Configure units if specified
	if units, ok := firstConfig["units"].([]interface{}); ok {
		cfg.InputConfig.Units = make([]string, len(units))
		for i, unit := range units {
			if unitStr, ok := unit.(string); ok {
				cfg.InputConfig.Units[i] = unitStr
			}
		}
	}

	// Set default priority to info
	cfg.InputConfig.Priority = "info"

	// Note: Storage for cursor persistence is optional
	// We can add this later if needed for production use

	return cfg, nil
}

func (t *translatorWithUnits) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)

	// Configure units from the provided units slice
	if len(t.units) > 0 {
		cfg.InputConfig.Units = make([]string, len(t.units))
		copy(cfg.InputConfig.Units, t.units)
	}

	// Set default priority to info
	cfg.InputConfig.Priority = "info"

	// Note: Storage for cursor persistence is optional
	// We can add this later if needed for production use

	return cfg, nil
}```

---

## File 6: translator/translate/otel/receiver/journald/translator_test.go
**Purpose**: Test patterns for receiver translator — note the StorageID nil assertion on line 82

```go
// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *journaldreceiver.JournaldConfig
		wantErr error
	}{
		"WithValidConfig": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"log_group_name":    "system-logs",
									"log_stream_name":   "{instance_id}",
									"retention_in_days": 7,
									"units":             []interface{}{"systemd", "kernel", "sshd"},
									"filters": []interface{}{
										map[string]interface{}{
											"type":       "exclude",
											"expression": ".*debug.*",
										},
									},
								},
							},
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: journald.Config{
					Units:    []string{"systemd", "kernel", "sshd"},
					Priority: "info",
				},
			},
		},
		"WithMissingConfig": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			translator := NewTranslator()
			got, err := translator.Translate(conf)

			if testCase.wantErr != nil {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)

			gotCfg, ok := got.(*journaldreceiver.JournaldConfig)
			require.True(t, ok)

			if testCase.want != nil {
				assert.Equal(t, testCase.want.InputConfig.Units, gotCfg.InputConfig.Units)
				assert.Equal(t, testCase.want.InputConfig.Priority, gotCfg.InputConfig.Priority)
				// Storage is optional and not configured by default
				assert.Nil(t, gotCfg.BaseConfig.StorageID)
			}
		})
	}
}```

---

## File 7: translator/translate/otel/common/common.go
**Purpose**: Defines the Translator interface, TranslatorMap, ComponentTranslator type alias, ComponentTranslators struct, and helper functions

```go
// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"container/list"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
)

const (
	AgentKey                                       = "agent"
	DebugKey                                       = "debug"
	MetricsKey                                     = "metrics"
	LogsKey                                        = "logs"
	TracesKey                                      = "traces"
	MetricsCollectedKey                            = "metrics_collected"
	LogsCollectedKey                               = "logs_collected"
	TracesCollectedKey                             = "traces_collected"
	MetricsDestinationsKey                         = "metrics_destinations"
	ECSKey                                         = "ecs"
	KubernetesKey                                  = "kubernetes"
	CloudWatchKey                                  = "cloudwatch"
	CloudWatchLogsKey                              = "cloudwatchlogs"
	PrometheusKey                                  = "prometheus"
	PrometheusConfigPathKey                        = "prometheus_config_path"
	AMPKey                                         = "amp"
	WorkspaceIDKey                                 = "workspace_id"
	EMFProcessorKey                                = "emf_processor"
	DisableMetricExtraction                        = "disable_metric_extraction"
	XrayKey                                        = "xray"
	OtlpKey                                        = "otlp"
	JmxKey                                         = "jmx"
	TLSKey                                         = "tls"
	Endpoint                                       = "endpoint"
	EndpointOverrideKey                            = "endpoint_override"
	RegionOverrideKey                              = "region_override"
	ProxyOverrideKey                               = "proxy_override"
	InsecureKey                                    = "insecure"
	LocalModeKey                                   = "local_mode"
	CredentialsKey                                 = "credentials"
	RoleARNKey                                     = "role_arn"
	SigV4Auth                                      = "sigv4auth"
	MetricsCollectionIntervalKey                   = "metrics_collection_interval"
	AggregationDimensionsKey                       = "aggregation_dimensions"
	MeasurementKey                                 = "measurement"
	DropOriginalMetricsKey                         = "drop_original_metrics"
	ForceFlushIntervalKey                          = "force_flush_interval"
	ContainerInsightsMetricGranularity             = "metric_granularity" // replaced with enhanced_container_insights
	EnhancedContainerInsights                      = "enhanced_container_insights"
	ResourcesKey                                   = "resources"
	PreferFullPodName                              = "prefer_full_pod_name"
	EnableAcceleratedComputeMetric                 = "accelerated_compute_metrics"
	AcceleratedComputeGPUMetricsCollectionInterval = "accelerated_compute_gpu_metrics_collection_interval"
	EnableKueueContainerInsights                   = "kueue_container_insights"
	AppendDimensionsKey                            = "append_dimensions"
	Console                                        = "console"
	DiskKey                                        = "disk"
	DiskIOKey                                      = "diskio"
	NetKey                                         = "net"
	Emf                                            = "emf"
	StructuredLog                                  = "structuredlog"
	ServiceAddress                                 = "service_address"
	Udp                                            = "udp"
	Tcp                                            = "tcp"
	TlsKey                                         = "tls"
	Tags                                           = "tags"
	Region                                         = "region"
	LogGroupName                                   = "log_group_name"
	LogStreamName                                  = "log_stream_name"
	NameKey                                        = "name"
	RenameKey                                      = "rename"
	UnitKey                                        = "unit"
	JournaldKey                                    = "journald"
)

const (
	CollectDMetricKey       = "collectd"
	CollectDPluginKey       = "socket_listener"
	CPUMetricKey            = "cpu"
	DiskMetricKey           = "disk"
	DiskIoMetricKey         = "diskio"
	StatsDMetricKey         = "statsd"
	SwapMetricKey           = "swap"
	MemMetricKey            = "mem"
	NetMetricKey            = "net"
	NetStatMetricKey        = "netstat"
	ProcessMetricKey        = "process"
	ProcStatMetricKey       = "procstat"
	SystemMetricsEnabledKey = "system_metrics_enabled"

	//Windows Plugins
	MemMetricKeyWindows          = "Memory"
	LogicalDiskMetricKeyWindows  = "LogicalDisk"
	NetworkMetricKeyWindows      = "Network Interface"
	PagingMetricKeyWindows       = "Paging"
	PhysicalDiskMetricKeyWindows = "PhysicalDisk"
	ProcessorMetricKeyWindows    = "Processor"
	SystemMetricKeyWindows       = "System"
	TCPv4MetricKeyWindows        = "TCPv4"
	TCPv6MetricKeyWindows        = "TCPv6"
)

const (
	PipelineNameHost                 = "host"
	PipelineNameHostCustomMetrics    = "hostCustomMetrics"
	PipelineNameHostDeltaMetrics     = "hostDeltaMetrics"
	PipelineNameHostOtlpMetrics      = "hostOtlpMetrics"
	PipelineNameContainerInsights    = "containerinsights"
	PipelineNameJmx                  = "jmx"
	PipelineNameContainerInsightsJmx = "containerinsightsjmx"
	PipelineNameEmfLogs              = "emf_logs"
	PipelineNamePrometheus           = "prometheus"
	PipelineNameKueue                = "kueueContainerInsights"
	PipelineNameSystemMetrics        = "systemmetrics"
	AppSignals                       = "application_signals"
	AppSignalsFallback               = "app_signals"
	AppSignalsRules                  = "rules"
)

const (
	DiskIOPrefix = "diskio_"
)

var (
	AppSignalsTraces          = ConfigKey(TracesKey, TracesCollectedKey, AppSignals)
	AppSignalsMetrics         = ConfigKey(LogsKey, MetricsCollectedKey, AppSignals)
	AppSignalsTracesFallback  = ConfigKey(TracesKey, TracesCollectedKey, AppSignalsFallback)
	AppSignalsMetricsFallback = ConfigKey(LogsKey, MetricsCollectedKey, AppSignalsFallback)

	AppSignalsConfigKeys = map[pipeline.Signal][]string{
		pipeline.SignalTraces:  {AppSignalsTraces, AppSignalsTracesFallback},
		pipeline.SignalMetrics: {AppSignalsMetrics, AppSignalsMetricsFallback},
	}
	SystemMetricsEnabledConfigKey = ConfigKey(AgentKey, SystemMetricsEnabledKey)
	JmxConfigKey                  = ConfigKey(MetricsKey, MetricsCollectedKey, JmxKey)
	ContainerInsightsConfigKey    = ConfigKey(LogsKey, MetricsCollectedKey, KubernetesKey)

	JmxTargets = []string{"activemq", "cassandra", "hbase", "hadoop", "jetty", "jvm", "kafka", "kafka-consumer", "kafka-producer", "solr", "tomcat", "wildfly"}

	AgentDebugConfigKey             = ConfigKey(AgentKey, DebugKey)
	MetricsAggregationDimensionsKey = ConfigKey(MetricsKey, AggregationDimensionsKey)
	OTLPLogsKey                     = ConfigKey(LogsKey, MetricsCollectedKey, OtlpKey)
	OTLPMetricsKey                  = ConfigKey(MetricsKey, MetricsCollectedKey, OtlpKey)
)

type TranslatorID interface {
	component.ID | pipeline.ID

	Name() string
}

// Translator is used to translate the JSON config into an
// OTEL config.
type Translator[C any, ID TranslatorID] interface {
	Translate(*confmap.Conf) (C, error)
	ID() ID
}

// TranslatorMap is a set of translators by their types.
type TranslatorMap[C any, ID TranslatorID] interface {
	// Set a translator to the map. If the ID is already present, replaces the translator.
	// Otherwise, adds it to the end of the list.
	Set(Translator[C, ID])
	// Get the translator for the component.ID.
	Get(ID) (Translator[C, ID], bool)
	// Merge another translator map in.
	Merge(TranslatorMap[C, ID])
	// Keys is the ordered component.IDs.
	Keys() []ID
	// Range iterates over each translator in order and calls the callback function on each.
	Range(func(Translator[C, ID]))
	// Len is the number of translators in the map.
	Len() int
}

type translatorMap[C any, ID TranslatorID] struct {
	// list stores the ordered translators.
	list *list.List
	// lookup stores the list.Elements containing the translators by ID.
	lookup map[ID]*list.Element
}

func (t translatorMap[C, ID]) Set(translator Translator[C, ID]) {
	if element, ok := t.lookup[translator.ID()]; ok {
		element.Value = translator
	} else {
		element = t.list.PushBack(translator)
		t.lookup[translator.ID()] = element
	}
}

func (t translatorMap[C, ID]) Get(id ID) (Translator[C, ID], bool) {
	element, ok := t.lookup[id]
	if !ok {
		return nil, ok
	}
	return element.Value.(Translator[C, ID]), ok
}

func (t translatorMap[C, ID]) Merge(other TranslatorMap[C, ID]) {
	if other != nil {
		other.Range(t.Set)
	}
}

func (t translatorMap[C, ID]) Keys() []ID {
	keys := make([]ID, 0, t.Len())
	t.Range(func(translator Translator[C, ID]) {
		keys = append(keys, translator.ID())
	})
	return keys
}

func (t translatorMap[C, ID]) Range(callback func(translator Translator[C, ID])) {
	for element := t.list.Front(); element != nil; element = element.Next() {
		callback(element.Value.(Translator[C, ID]))
	}
}

func (t translatorMap[C, ID]) Len() int {
	return t.list.Len()
}

// NewTranslatorMap creates a TranslatorMap from the translators.
func NewTranslatorMap[C any, ID TranslatorID](translators ...Translator[C, ID]) TranslatorMap[C, ID] {
	t := translatorMap[C, ID]{
		list:   list.New(),
		lookup: make(map[ID]*list.Element, len(translators)),
	}
	for _, translator := range translators {
		t.Set(translator)
	}
	return t
}

type ID interface {
	String() string
}

// A MissingKeyError occurs when a translator is used for a JSON
// config that does not have a required key. This typically means
// that the pipeline was configured incorrectly.
type MissingKeyError struct {
	ID      ID
	JsonKey string
}

func (e *MissingKeyError) Error() string {
	return fmt.Sprintf("%q missing key in JSON: %q", e.ID, e.JsonKey)
}

// ComponentTranslator is a Translator that converts a JSON config into a component
type ComponentTranslator = Translator[component.Config, component.ID]

// ComponentTranslatorMap is a map-like container which stores ComponentTranslators
type ComponentTranslatorMap = TranslatorMap[component.Config, component.ID]

// ComponentTranslators is a component ID and respective service pipeline.
type ComponentTranslators struct {
	Receivers  ComponentTranslatorMap
	Processors ComponentTranslatorMap
	Exporters  ComponentTranslatorMap
	Extensions ComponentTranslatorMap
}

// PipelineTranslator is a Translator that converts a JSON config into a pipeline
type PipelineTranslator = Translator[*ComponentTranslators, pipeline.ID]

// PipelineTranslatorMap is a map-like container which stores PipelineTranslators
type PipelineTranslatorMap = TranslatorMap[*ComponentTranslators, pipeline.ID]

// ConfigKey joins the keys separated by confmap.KeyDelimiter.
// This helps translators navigate the confmap.Conf that the
// JSON config is loaded into.
func ConfigKey(keys ...string) string {
	return strings.Join(keys, confmap.KeyDelimiter)
}

// ParseDuration attempts to parse the input into a duration.
// Returns a zero duration and an error if invalid.
func ParseDuration(v interface{}) (time.Duration, error) {
	if v != nil {
		if fv, ok := v.(float64); ok {
			return time.Second * time.Duration(fv), nil
		}
		s, ok := v.(string)
		if !ok {
			s = fmt.Sprintf("%v", v)
		}
		duration, err := time.ParseDuration(s)
		if err == nil {
			return duration, nil
		}
		sI, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			return time.Second * time.Duration(sI), nil
		}
		sF, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return time.Second * time.Duration(sF), nil
		}
	}
	return time.Duration(0), fmt.Errorf("invalid type %v", reflect.TypeOf(v))
}

// GetString gets the string value for the key. If the key is missing,
// ok will be false.
func GetString(conf *confmap.Conf, key string) (string, bool) {
	if value := conf.Get(key); value != nil {
		got, ok := value.(string)
		// if the value isn't a string, convert it
		if !ok {
			got = fmt.Sprintf("%v", value)
			ok = true
		}
		return got, ok
	}
	return "", false
}

// GetArray gets the array value for the key. If the key is missing,
// the return value will be nil
func GetArray[C any](conf *confmap.Conf, key string) []C {
	if value := conf.Get(key); value != nil {
		var arr []C
		got, _ := value.([]any)
		for _, entry := range got {
			if t, ok := entry.(C); ok {
				arr = append(arr, t)
			}
		}
		return arr
	}
	return nil
}

// GetBool gets the bool value for the key. If the key is missing or the
// value is not a bool type, then ok will be false.
func GetBool(conf *confmap.Conf, key string) (value bool, ok bool) {
	if v := conf.Get(key); v != nil {
		value, ok = v.(bool)
	}
	return
}

// GetOrDefaultBool gets the bool value for the key. If the key is missing or the
// value is not a bool type, then the defaultVal is returned.
func GetOrDefaultBool(conf *confmap.Conf, key string, defaultVal bool) bool {
	if v := conf.Get(key); v != nil {
		if val, ok := v.(bool); ok {
			return val
		}
	}
	return defaultVal
}

// GetNumber gets the number value for the key. The switch works through
// all reasonable number types (the default is typically float64)
func GetNumber(conf *confmap.Conf, key string) (float64, bool) {
	if v := conf.Get(key); v != nil {
		switch i := v.(type) {
		case float64:
			return i, true
		case float32:
			return float64(i), true
		case int64:
			return float64(i), true
		case int32:
			return float64(i), true
		case int:
			return float64(i), true
		case uint64:
			return float64(i), true
		case uint32:
			return float64(i), true
		case uint:
			return float64(i), true
		case string:
		}
	}
	return 0, false
}

// GetOrDefaultNumber gets the number value for the key. If the key is missing or the
// value is not a number type, then the defaultVal is returned.
func GetOrDefaultNumber(conf *confmap.Conf, key string, defaultVal float64) float64 {
	value, ok := GetNumber(conf, key)
	if !ok {
		return defaultVal
	}
	return value
}

// GetDuration gets the value for the key and calls ParseDuration on it.
// If the key is missing, it is unable to parse the duration, or the
// duration is set to 0, then the returned bool will be false.
func GetDuration(conf *confmap.Conf, key string) (time.Duration, bool) {
	var duration time.Duration
	var ok bool
	if value := conf.Get(key); value != nil {
		var err error
		duration, err = ParseDuration(value)
		ok = err == nil && duration > 0
	}
	return duration, ok
}

// GetOrDefaultDuration from the first section in the keychain with a
// parsable duration. If none are found, returns the defaultDuration.
func GetOrDefaultDuration(conf *confmap.Conf, keychain []string, defaultDuration time.Duration) time.Duration {
	for _, key := range keychain {
		duration, ok := GetDuration(conf, key)
		if !ok {
			continue
		}
		return duration
	}
	return defaultDuration
}

func GetYamlFileToYamlConfig(cfg interface{}, yamlFile string) (interface{}, error) {
	var cfgMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlFile), &cfgMap); err != nil {
		return nil, fmt.Errorf("unable to read default config: %w", err)
	}

	conf := confmap.NewFromStringMap(cfgMap)
	if err := conf.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal config: %w", err)
	}
	return cfg, nil
}

// GetIndexedMap gets the sub map based on the config key and index. If the config value is an array, then the value
// at the index is returned. If it is a map, then the index is ignored and the map is returned directly.
func GetIndexedMap(conf *confmap.Conf, configKey string, index int) map[string]any {
	var got map[string]any
	switch v := conf.Get(configKey).(type) {
	case []any:
		if index != -1 && len(v) > index {
			got = v[index].(map[string]any)
		}
	case map[string]any:
		got = v
	}
	return got
}

// GetMeasurements gets the string values in the measurements section of the provided map. If there are metric
// decoration elements, includes the value associated with the "name" key.
func GetMeasurements(m map[string]any) []string {
	var results []string
	if measurements, ok := m[MeasurementKey].([]any); ok {
		for _, measurement := range measurements {
			switch v := measurement.(type) {
			case string:
				results = append(results, v)
			case map[string]any:
				if n, ok := v[NameKey]; ok {
					if s, ok := n.(string); ok {
						results = append(results, s)
					}
				}
			}
		}
	}
	return results
}

// IsAnySet checks if any of the provided keys are present in the configuration.
func IsAnySet(conf *confmap.Conf, keys []string) bool {
	for _, key := range keys {
		if conf.IsSet(key) {
			return true
		}
	}
	return false
}

func KueueContainerInsightsEnabled(conf *confmap.Conf) bool {
	return GetOrDefaultBool(conf, ConfigKey(LogsKey, MetricsCollectedKey, KubernetesKey, EnableKueueContainerInsights), false)
}

func GetClusterName(conf *confmap.Conf) string {
	val, ok := GetString(conf, ConfigKey(LogsKey, MetricsCollectedKey, KubernetesKey, "cluster_name"))
	if ok && val != "" {
		return val
	}

	envVarClusterName := os.Getenv("K8S_CLUSTER_NAME")
	if envVarClusterName != "" {
		return envVarClusterName
	}

	return util.GetClusterNameFromEc2Tagger()
}
```

---

## File 8: service/defaultcomponents/components.go (extension registration excerpt, lines 166-190)
**Purpose**: Shows `filestorage.NewFactory()` is ALREADY registered — no changes needed here

```go
	if factories.Extensions, err = otelcol.MakeFactoryMap[extension.Factory](
		agenthealth.NewFactory(),
		awsproxy.NewFactory(),
		entitystore.NewFactory(),
		k8smetadata.NewFactory(),
		nodemetadatacache.NewFactory(),
		server.NewFactory(),
		ecsobserver.NewFactory(),
		filestorage.NewFactory(),
		healthcheckextension.NewFactory(),
		pprofextension.NewFactory(),
		sigv4authextension.NewFactory(),
		zpagesextension.NewFactory(),
	); err != nil {
		return otelcol.Factories{}, err
	}

	return factories, nil
}
```
