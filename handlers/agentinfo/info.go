// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentinfo

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
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
	versionFilename = "CWAGENT_VERSION"
	idFilename      = "CWAGENT_ID"
	unknownVersion  = "Unknown"
	placeholder     = "-"
	updateInterval  = time.Minute
	okStatusCode    = "200"
)

var components string

type AgentInfo interface {
	FullVersion() string
	UserAgent() string
	Version() string
	RecordOpData(time.Duration, int, error)
}

type agentInfo struct {
	proc          *process.Process
	version       string
	fullVersion   string
	id            string
	procTelemetry string
	userAgent     string
	nextUpdate    time.Time
}

func init() {
	SetComponents(&otelcol.Config{}, &config.Config{}) // sets placeholder for components
}

func New() AgentInfo {
	return newAgentInfo()
}

func newAgentInfo() *agentInfo {
	ai := new(agentInfo)

	ai.version = readVersionFile()
	ai.fullVersion = fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		ai.version,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
	ai.id = readOrWriteUUIDFile()

	if ai.userAgent = os.Getenv(envconfig.CWAGENT_USER_AGENT); ai.userAgent == "" {
		ai.userAgent = fmt.Sprintf("%s ID/%s", ai.fullVersion, ai.id)
		if usageData, err := strconv.ParseBool(os.Getenv(envconfig.CWAGENT_USAGE_DATA)); err != nil || usageData {
			// agent telemetry is enabled
			ai.proc, _ = process.NewProcess(int32(os.Getpid()))
		}
	}

	return ai
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

func (ai *agentInfo) RecordOpData(latency time.Duration, sendCount int, err error) {
	if ai.isTelemetryDisabled() { //
		return
	}

	code := okStatusCode

	if err != nil {
		code = placeholder
		if reqErr, ok := err.(awserr.RequestFailure); ok {
			code = strconv.Itoa(reqErr.StatusCode())
		}
	}

	opTelemetry := fmt.Sprintf("%d %d %s",
		latency,
		sendCount,
		code)
	if now := time.Now(); now.After(ai.nextUpdate) {
		ai.procTelemetry = fmt.Sprintf("Telemetry: %s %s %s %s",
			telemetryToStr(ai.cpuPercent()),
			telemetryToStr(ai.memBytes()),
			telemetryToStr(ai.fileDescriptorCount()),
			telemetryToStr(ai.threadCount()))
		ai.nextUpdate = now.Add(updateInterval)
	}
	ai.userAgent = fmt.Sprintf("%s ID/%s (%s; %s %s)", ai.fullVersion, ai.id, components, ai.procTelemetry, opTelemetry)
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

func (ai *agentInfo) isTelemetryDisabled() bool {
	return ai.proc == nil
}

func SetComponents(otelcfg *otelcol.Config, telegrafcfg *config.Config) {
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

	components = fmt.Sprintf("Receivers: %s; Processors: %s; Exporters: %s",
		componentsToStr(receivers),
		componentsToStr(processors),
		componentsToStr(exporters))
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

func componentsToStr(components []string) string {
	if len(components) == 0 {
		return placeholder
	}
	return strings.Join(components, " ")
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
