// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentinfo

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/google/uuid"
	"github.com/influxdata/telegraf/config"
	"github.com/shirou/gopsutil/v3/process"
	"go.opentelemetry.io/collector/otelcol"
	"golang.org/x/exp/maps"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
)

const (
	versionFilename = "CWAGENT_VERSION"
	unknownVersion  = "Unknown"
	updateInterval  = time.Minute
)

var (
	receivers               string
	processors              string
	exporters               string
	usageDataEnabled        bool
	onceUsageData           sync.Once
	containerInsightsRegexp = regexp.MustCompile("^/aws/.*containerinsights/.*/(performance|prometheus)$")
	version                 = readVersionFile()
	fullVersion             = getFullVersion(version)
	id                      = uuid.NewString()
	sharedConfigFallback    atomic.Bool
	imdsFallbackSucceed     atomic.Bool
)

var isRunningAsRoot = defaultIsRunningAsRoot

type AgentInfo interface {
	RecordOpData(time.Duration, int, error)
	StatsHeader() string
	UserAgent() string
}

type agentInfo struct {
	proc        *process.Process
	nextUpdate  time.Time
	statsHeader string
	userAgent   string
}

type agentStats struct {
	CpuPercent           *float64 `json:"cpu,omitempty"`
	MemoryBytes          *uint64  `json:"mem,omitempty"`
	FileDescriptorCount  *int32   `json:"fd,omitempty"`
	ThreadCount          *int32   `json:"th,omitempty"`
	LatencyMillis        *int64   `json:"lat,omitempty"`
	PayloadBytes         *int     `json:"load,omitempty"`
	StatusCode           *int     `json:"code,omitempty"`
	SharedConfigFallback *int     `json:"scfb,omitempty"`
	ImdsFallbackSucceed  *int     `json:"ifs,omitempty"`
}

func New(groupName string) AgentInfo {
	return newAgentInfo(groupName)
}

func newAgentInfo(groupName string) *agentInfo {
	ai := new(agentInfo)
	ai.userAgent = getUserAgent(groupName, fullVersion, receivers, processors, exporters, isUsageDataEnabled())
	if isUsageDataEnabled() {
		ai.proc, _ = process.NewProcess(int32(os.Getpid()))
		if ai.proc == nil {
			return ai
		}
		stats := agentStats{
			CpuPercent:          ai.cpuPercent(),
			MemoryBytes:         ai.memoryBytes(),
			FileDescriptorCount: ai.fileDescriptorCount(),
			ThreadCount:         ai.threadCount(),
		}
		ai.statsHeader = getAgentStats(stats)
		ai.nextUpdate = time.Now().Add(updateInterval)
	}

	return ai
}

func (ai *agentInfo) UserAgent() string {
	return ai.userAgent
}

func (ai *agentInfo) RecordOpData(latency time.Duration, payloadBytes int, err error) {
	if ai.proc == nil {
		return
	}

	stats := agentStats{
		LatencyMillis: aws.Int64(latency.Milliseconds()),
		PayloadBytes:  aws.Int(payloadBytes),
		StatusCode:    getStatusCode(err),
	}

	if now := time.Now(); now.After(ai.nextUpdate) {
		stats.CpuPercent = ai.cpuPercent()
		stats.MemoryBytes = ai.memoryBytes()
		stats.FileDescriptorCount = ai.fileDescriptorCount()
		stats.ThreadCount = ai.threadCount()
		stats.SharedConfigFallback = getSharedConfigFallback()
		stats.ImdsFallbackSucceed = succeedImdsFallback()
		ai.nextUpdate = now.Add(updateInterval)
	}

	ai.statsHeader = getAgentStats(stats)
}

func (ai *agentInfo) StatsHeader() string {
	return ai.statsHeader
}

func (ai *agentInfo) cpuPercent() *float64 {
	if cpuPercent, err := ai.proc.CPUPercent(); err == nil {
		return aws.Float64(float64(int64(cpuPercent*10)) / 10) // truncate to 10th decimal place
	}
	return nil
}

func (ai *agentInfo) memoryBytes() *uint64 {
	if memInfo, err := ai.proc.MemoryInfo(); err == nil {
		return aws.Uint64(memInfo.RSS)
	}
	return nil
}

func (ai *agentInfo) fileDescriptorCount() *int32 {
	if fdCount, err := ai.proc.NumFDs(); err == nil {
		return aws.Int32(fdCount)
	}
	return nil
}

// we only need to know if value is 1
// thus return nil if not set
func succeedImdsFallback() *int {
	if imdsFallbackSucceed.Load() {
		return aws.Int(1)
	}
	return nil
}

func (ai *agentInfo) threadCount() *int32 {
	if thCount, err := ai.proc.NumThreads(); err == nil {
		return aws.Int32(thCount)
	}
	return nil
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

	if !isRunningAsRoot() {
		receiverSet.Add("run_as_user")
	}

	receiversSlice := maps.Keys(receiverSet)
	processorsSlice := maps.Keys(processorSet)
	exportersSlice := maps.Keys(exporterSet)

	sort.Strings(receiversSlice)
	sort.Strings(processorsSlice)
	sort.Strings(exportersSlice)

	receivers = strings.Join(receiversSlice, " ")
	processors = strings.Join(processorsSlice, " ")
	exporters = strings.Join(exportersSlice, " ")
}

func Version() string {
	return version
}

func FullVersion() string {
	return fullVersion
}

func getAgentStats(stats agentStats) string {
	raw, err := json.Marshal(stats)
	if err != nil {
		log.Printf("W! Failed to serialize agent stats, error: %s", err)
		return ""
	}
	content := strings.TrimPrefix(string(raw), "{")
	return strings.TrimSuffix(content, "}")
}

func getStatusCode(err error) *int {
	if err == nil {
		return aws.Int(http.StatusOK)
	}
	if reqErr, ok := err.(awserr.RequestFailure); ok {
		return aws.Int(reqErr.StatusCode())
	}
	return nil
}

func getUserAgent(groupName, fullVersion, receivers, processors, exporters string, usageDataEnabled bool) string {
	if ua := os.Getenv(envconfig.CWAGENT_USER_AGENT); ua != "" {
		return ua
	}
	if !usageDataEnabled {
		return fullVersion
	}

	outputs := strings.Clone(exporters)
	if outputs != "" && containerInsightsRegexp.MatchString(groupName) && !strings.Contains(outputs, "container_insights") {
		outputs += " container_insights"
	}

	components := make([]string, 0, 0)
	if receivers != "" {
		components = append(components, fmt.Sprintf("inputs:(%s)", receivers))
	}
	if processors != "" {
		components = append(components, fmt.Sprintf("processors:(%s)", processors))
	}
	if outputs != "" {
		components = append(components, fmt.Sprintf("outputs:(%s)", outputs))
	}

	return strings.TrimSpace(fmt.Sprintf("%s ID/%s %s", fullVersion, id, strings.Join(components, " ")))
}

func getFullVersion(version string) string {
	return fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		version,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
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

// this returns true for true or invalid
// examples of invalid are not set env var, "", "invalid"
func getUsageDataEnabled() bool {
	ok, err := strconv.ParseBool(os.Getenv(envconfig.CWAGENT_USAGE_DATA))
	return ok || err != nil
}

func isUsageDataEnabled() bool {
	onceUsageData.Do(func() {
		usageDataEnabled = getUsageDataEnabled()
	})
	return usageDataEnabled
}

func defaultIsRunningAsRoot() bool {
	return os.Getuid() == 0
}

func RecordSharedConfigFallback() {
	sharedConfigFallback.Store(true)
}

func getSharedConfigFallback() *int {
	if sharedConfigFallback.Load() {
		return aws.Int(1)
	}
	return nil
}

func SetImdsFallbackSucceed() {
	imdsFallbackSucceed.Store(true)
}
