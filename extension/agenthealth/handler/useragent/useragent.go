// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	telegraf "github.com/influxdata/telegraf/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/atomic"
	"golang.org/x/exp/maps"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/internal/version"
	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
)

const (
	flagRunAsUser                 = "run_as_user"
	flagContainerInsights         = "container_insights"
	flagAppSignals                = "application_signals"
	flagEnhancedContainerInsights = "enhanced_container_insights"

	separator = " "

	typeInputs     = "inputs"
	typeProcessors = "processors"
	typeOutputs    = "outputs"
)

var (
	singleton UserAgent
	once      sync.Once
)

type UserAgent interface {
	SetComponents(otelCfg *otelcol.Config, telegrafCfg *telegraf.Config)
	SetContainerInsightsFlag()
	Header(isUsageDataEnabled bool) string
	Listen(listener func())
}

type userAgent struct {
	dataLock sync.Mutex
	id       string

	listenerLock sync.Mutex
	listeners    []func()
	isRoot       bool

	inputs     collections.Set[string]
	processors collections.Set[string]
	outputs    collections.Set[string]

	inputsStr     atomic.String
	processorsStr atomic.String
	outputsStr    atomic.String
}

var _ UserAgent = (*userAgent)(nil)

func (ua *userAgent) SetComponents(otelCfg *otelcol.Config, telegrafCfg *telegraf.Config) {
	for _, input := range telegrafCfg.Inputs {
		ua.inputs.Add(input.Config.Name)
	}
	for _, output := range telegrafCfg.Outputs {
		ua.outputs.Add(output.Config.Name)
	}

	for _, pipeline := range otelCfg.Service.Pipelines {
		for _, receiver := range pipeline.Receivers {
			// trim the adapter prefix from adapted Telegraf plugins
			name := strings.TrimPrefix(receiver.Type().String(), adapter.TelegrafPrefix)
			ua.inputs.Add(name)
		}
		for _, processor := range pipeline.Processors {
			ua.processors.Add(processor.Type().String())
		}
		for _, exporter := range pipeline.Exporters {
			ua.outputs.Add(exporter.Type().String())
			if exporter.Type().String() == "awsemf" {
				cfg := otelCfg.Exporters[exporter].(*awsemfexporter.Config)
				if cfg.IsAppSignalsEnabled() {
					ua.outputs.Add(flagAppSignals)
					agent.UsageFlags().Set(agent.FlagAppSignal)
				}
				if cfg.IsEnhancedContainerInsights() {
					ua.outputs.Add(flagEnhancedContainerInsights)
					agent.UsageFlags().Set(agent.FlagEnhancedContainerInsights)
				}
			}
		}
	}

	if !ua.isRoot {
		ua.inputs.Add(flagRunAsUser)
	}

	ua.inputsStr.Store(componentsStr(typeInputs, ua.inputs))
	ua.processorsStr.Store(componentsStr(typeProcessors, ua.processors))
	ua.outputsStr.Store(componentsStr(typeOutputs, ua.outputs))
	ua.notify()
}

func (ua *userAgent) SetContainerInsightsFlag() {
	ua.dataLock.Lock()
	defer ua.dataLock.Unlock()
	if !ua.outputs.Contains(flagContainerInsights) {
		ua.outputs.Add(flagContainerInsights)
		ua.outputsStr.Store(componentsStr(typeOutputs, ua.outputs))
		ua.notify()
	}
}

func (ua *userAgent) Listen(listener func()) {
	ua.listenerLock.Lock()
	defer ua.listenerLock.Unlock()
	ua.listeners = append(ua.listeners, listener)
}

func (ua *userAgent) notify() {
	ua.listenerLock.Lock()
	defer ua.listenerLock.Unlock()
	for _, listener := range ua.listeners {
		listener()
	}
}

func (ua *userAgent) Header(isUsageDataEnabled bool) string {
	if envUserAgent := os.Getenv(envconfig.CWAGENT_USER_AGENT); envUserAgent != "" {
		return envUserAgent
	}
	if !isUsageDataEnabled {
		return version.Full()
	}

	var components []string
	inputs := ua.inputsStr.Load()
	if inputs != "" {
		components = append(components, inputs)
	}
	processors := ua.processorsStr.Load()
	if processors != "" {
		components = append(components, processors)
	}
	outputs := ua.outputsStr.Load()
	if outputs != "" {
		components = append(components, outputs)
	}

	return strings.TrimSpace(fmt.Sprintf("%s ID/%s %s", version.Full(), ua.id, strings.Join(components, separator)))
}

func componentsStr(componentType string, componentSet collections.Set[string]) string {
	if len(componentSet) == 0 {
		return ""
	}
	components := maps.Keys(componentSet)
	sort.Strings(components)
	return fmt.Sprintf("%s:(%s)", componentType, strings.Join(components, separator))
}

func isRunningAsRoot() bool {
	return os.Getuid() == 0
}

func newUserAgent() *userAgent {
	return &userAgent{
		id:         uuid.NewString(),
		isRoot:     isRunningAsRoot(),
		inputs:     collections.NewSet[string](),
		processors: collections.NewSet[string](),
		outputs:    collections.NewSet[string](),
	}
}

func Get() UserAgent {
	once.Do(func() {
		singleton = newUserAgent()
	})
	return singleton
}
