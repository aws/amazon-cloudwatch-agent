// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentinfo

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/influxdata/telegraf/config"
	"github.com/shirou/gopsutil/v3/process"
	"go.opentelemetry.io/collector/otelcol"
	"golang.org/x/exp/maps"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/cfg/envconfig"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/receiver/adapter"
)

const (
	versionFilename   = "CWAGENT_VERSION"
	idFilename        = "CWAGENT_ID"
	unknownVersion    = "Unknown"
	placeholder       = "-"
	telemetryInterval = time.Minute
)

var defaultAgentInfo = newAgentInfo()

type AgentInfo interface {
	FullVersion() string
	SetPlugins(*otelcol.Config, *config.Config)
	Shutdown()
	Start()
	UserAgent() string
	Version() string
}

type agentInfo struct {
	proc        *process.Process
	version     string
	fullVersion string
	plugins     string
	telemetry   string
	userAgent   string
	id          string
	done        chan struct{}
}

func Get() AgentInfo {
	return defaultAgentInfo
}

func newAgentInfo() *agentInfo {
	ai := new(agentInfo)
	ai.proc, _ = process.NewProcess(int32(os.Getpid()))
	ai.version = readVersionFile()
	ai.fullVersion = fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		ai.version,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
	ai.id = readOrWriteUUIDFile()
	ai.userAgent = ai.createUserAgent()

	return ai
}

func (ai *agentInfo) Start() {
	if ai.proc == nil {
		return
	}

	ai.telemetry = ai.createTelemetry()
	ai.userAgent = ai.createUserAgentWithTelemetry()
	ai.done = make(chan struct{})
	go func() {
		ticker := time.NewTicker(telemetryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ai.telemetry = ai.createTelemetry()
				ai.userAgent = ai.createUserAgentWithTelemetry()
			case <-ai.done:
				return
			}
		}
	}()
}

func (ai *agentInfo) Shutdown() {
	if ai.proc == nil {
		return
	}

	close(ai.done)
}

func (ai *agentInfo) Version() string {
	return ai.version
}

func (ai *agentInfo) FullVersion() string {
	return ai.fullVersion
}

func (ai *agentInfo) UserAgent() string {
	return ai.userAgent
}

func (ai *agentInfo) SetPlugins(otelcfg *otelcol.Config, telegrafcfg *config.Config) {
	receiverSet := collections.NewSet[string]()
	processorSet := collections.NewSet[string]()
	exporterSet := collections.NewSet[string]()

	for _, input := range telegrafcfg.Inputs {
		receiverSet.Add(input.Config.Name)
	}
	for _, output := range telegrafcfg.Outputs {
		exporterSet.Add(output.Config.Name)
	}

	for _, pipeline := range otelcfg.Service.Pipelines {
		for _, receiver := range pipeline.Receivers {
			// trim the adapter prefix from adapted Telegraf plugins
			name := strings.TrimPrefix(string(receiver.Type()), adapter.TelegrafPrefix)
			receiverSet.Add(name)
		}
		for _, processor := range pipeline.Processors {
			processorSet.Add(string(processor.Type()))
		}
		for _, exporter := range pipeline.Exporters {
			exporterSet.Add(string(exporter.Type()))
		}
	}

	receivers := maps.Keys(receiverSet)
	processors := maps.Keys(processorSet)
	exporters := maps.Keys(exporterSet)

	sort.Strings(receivers)
	sort.Strings(processors)
	sort.Strings(exporters)

	receiversStr := strings.Join(receivers, " ")
	processorsStr := strings.Join(processors, " ")
	exportersStr := strings.Join(exporters, " ")

	ai.plugins = fmt.Sprintf("Inputs:%s; Processors:%s; Outputs:%s", receiversStr, processorsStr, exportersStr)
	ai.userAgent = ai.createUserAgent()
}

func (ai *agentInfo) cpuPercent() (float64, error) {
	return ai.proc.CPUPercent()
}

func (ai *agentInfo) memBytes() (uint64, error) {
	memInfo, err := ai.proc.MemoryInfo()
	if err != nil {
		return 0, err
	}
	return memInfo.RSS, nil
}

func (ai *agentInfo) fileDescriptorCount() (uint64, error) {
	fdCount, err := ai.proc.NumFDs()
	return uint64(fdCount), err
}

func (ai *agentInfo) threadCount() (uint64, error) {
	thCount, err := ai.proc.NumThreads()
	return uint64(thCount), err
}

func (ai *agentInfo) createTelemetry() string {
	return fmt.Sprintf("Telemetry: %s %s %s %s",
		telemetryToStr(ai.cpuPercent()),
		telemetryToStr(ai.memBytes()),
		telemetryToStr(ai.fileDescriptorCount()),
		telemetryToStr(ai.threadCount()))
}

func (ai *agentInfo) createUserAgent() string {
	if ua := os.Getenv(envconfig.CWAGENT_USER_AGENT); ua != "" {
		return ua
	}

	if ai.proc == nil {
		return fmt.Sprintf("%s ID/%s (%s)", ai.fullVersion, ai.id, ai.plugins)
	}

	return ai.createUserAgentWithTelemetry()
}

func (ai *agentInfo) createUserAgentWithTelemetry() string {
	return fmt.Sprintf("%s ID/%s (%s; %s)", ai.fullVersion, ai.id, ai.plugins, ai.telemetry)
}

func telemetryToStr[V uint64 | float64](num V, err error) string {
	if err != nil {
		return placeholder
	}

	switch n := any(num).(type) {
	case uint64:
		return fmt.Sprintf("%d", n)
	case float64:
		return fmt.Sprintf("%.1f", n)
	}

	return placeholder
}

func readVersionFile() string {
	ex, err := os.Executable()
	if err != nil {
		return unknownVersion
	}

	versionFilePath := filepath.Join(filepath.Dir(ex), versionFilename)
	if _, err := os.Stat(versionFilePath); err != nil {
		return unknownVersion
	}

	byteArray, err := os.ReadFile(versionFilePath)
	if err != nil {
		return unknownVersion
	}

	return strings.Trim(string(byteArray), " \n\r\t")
}

func readOrWriteUUIDFile() string {
	ex, err := os.Executable()
	if err != nil {
		return uuid.NewString()
	}

	idFilePath := filepath.Join(filepath.Dir(ex), idFilename)
	if _, err := os.Stat(idFilePath); err != nil {
		return createAndWriteUUIDFile(idFilePath)
	}

	byteArray, err := os.ReadFile(idFilePath)
	if err != nil {
		return createAndWriteUUIDFile(idFilePath)
	}

	id, err := uuid.ParseBytes(byteArray)
	if err != nil {
		return createAndWriteUUIDFile(idFilePath)
	}

	return id.String()
}

func createAndWriteUUIDFile(idFilePath string) string {
	id := uuid.NewString()
	_ = os.WriteFile(idFilePath, []byte(id), 0644)

	return id
}
