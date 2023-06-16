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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/google/uuid"
	"github.com/influxdata/telegraf/config"
	"github.com/shirou/gopsutil/v3/process"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

const (
	versionFilename = "CWAGENT_VERSION"
	unknownVersion  = "Unknown"
	updateInterval  = time.Minute
)

var (
	BuildStr                string
	inputs                  string
	outputs                 string
	usageDataEnabled        bool
	onceUsageData           sync.Once
	containerInsightsRegexp = regexp.MustCompile("^/aws/.*containerinsights/.*/(performance|prometheus)$")
	version                 = readVersionFile()
	fullVersion             = getFullVersion(version, BuildStr)
	id                      = uuid.NewString()
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
	stats       agentStats
	statsHeader string
	userAgent   string
}

type agentStats struct {
	CpuPercent          *float64       `json:"cpu,omitempty"`
	MemoryBytes         *uint64        `json:"mem,omitempty"`
	FileDescriptorCount *int32         `json:"fd,omitempty"`
	ThreadCount         *int32         `json:"th,omitempty"`
	LatencyMillis       *time.Duration `json:"lat,omitempty"`
	PayloadBytes        *int           `json:"load,omitempty"`
	StatusCode          *int           `json:"code,omitempty"`
}

func New(groupName string) AgentInfo {
	return newAgentInfo(groupName)
}

func newAgentInfo(groupName string) *agentInfo {
	ai := new(agentInfo)
	ai.userAgent = getUserAgent(groupName, fullVersion, inputs, outputs, isUsageDataEnabled())
	if isUsageDataEnabled() {
		ai.proc, _ = process.NewProcess(int32(os.Getpid()))
		if ai.proc == nil {
			return ai
		}
		ai.stats = agentStats{
			CpuPercent:          ai.cpuPercent(),
			MemoryBytes:         ai.memoryBytes(),
			FileDescriptorCount: ai.fileDescriptorCount(),
			ThreadCount:         ai.threadCount(),
		}
		ai.statsHeader = getAgentStats(ai.stats)
		ai.nextUpdate = time.Now().Add(updateInterval)
	}

	return ai
}

func (ai *agentInfo) UserAgent() string {
	return ai.userAgent
}

func (ai *agentInfo) RecordOpData(latencyMillis time.Duration, payloadBytes int, err error) {
	if ai.proc == nil {
		return
	}

	ai.stats.LatencyMillis = &latencyMillis
	ai.stats.PayloadBytes = aws.Int(payloadBytes)
	ai.stats.StatusCode = getStatusCode(err)

	if now := time.Now(); now.After(ai.nextUpdate) {
		ai.stats.CpuPercent = ai.cpuPercent()
		ai.stats.MemoryBytes = ai.memoryBytes()
		ai.stats.FileDescriptorCount = ai.fileDescriptorCount()
		ai.stats.ThreadCount = ai.threadCount()
		ai.nextUpdate = now.Add(updateInterval)
	}

	ai.statsHeader = getAgentStats(ai.stats)
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

func (ai *agentInfo) threadCount() *int32 {
	if thCount, err := ai.proc.NumThreads(); err == nil {
		return aws.Int32(thCount)
	}
	return nil
}

func SetComponents(cfg *config.Config) {
	outputs = strings.Join(cfg.OutputNames(), " ")
	inputs = strings.Join(cfg.InputNames(), " ")

	if !isRunningAsRoot() {
		inputs += " run_as_user" // `inputs` is never empty, or agent will not start
	}
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

func getUserAgent(groupName, fullVersion, inputs, outputs string, usageDataEnabled bool) string {
	if ua := os.Getenv(envconfig.CWAGENT_USER_AGENT); ua != "" {
		return ua
	}
	if !usageDataEnabled {
		return fullVersion
	}

	outputsClone := strings.Clone(outputs)
	if outputsClone != "" && containerInsightsRegexp.MatchString(groupName) && !strings.Contains(outputsClone, "container_insights") {
		outputsClone += " container_insights"
	}

	components := make([]string, 0, 0)
	if inputs != "" {
		components = append(components, fmt.Sprintf("inputs:(%s)", inputs))
	}
	if outputsClone != "" {
		components = append(components, fmt.Sprintf("outputs:(%s)", outputsClone))
	}

	return strings.TrimSpace(fmt.Sprintf("%s ID/%s %s", fullVersion, id, strings.Join(components, " ")))
}

func getFullVersion(version, build string) string {
	return strings.TrimSpace(fmt.Sprintf("CWAgent/%s (%s; %s; %s) %s",
		version,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		build))
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
