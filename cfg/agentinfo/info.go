// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentinfo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/influxdata/telegraf/config"
	"go.opentelemetry.io/collector/otelcol"
	"golang.org/x/exp/maps"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/cfg/envconfig"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/receiver/adapter"
)

const (
	containerInsightRegexp = "^/aws/.*containerinsights/.*/(performance|prometheus)$"
	versionFilename        = "CWAGENT_VERSION"
	// We will fall back to a major version if no valid version file is found
	fallbackVersion = "1"
)

var isRunningAsRoot = func() bool {
	return os.Getuid() == 0
}

var (
	VersionStr string
	BuildStr   string = "No Build Date"
	receivers  []string
	processors []string
	exporters  []string

	userAgentMap        = make(map[string]string)
	ciCompiledRegexp, _ = regexp.Compile(containerInsightRegexp)
)

func Version() string {
	if VersionStr != "" {
		return VersionStr
	}

	version, err := readVersionFile()
	if err != nil {
		return fallbackVersion
	}

	VersionStr = version
	return version
}

func Build() string {
	return BuildStr
}

func Plugins(groupName string) string {
	receiversStr := strings.Join(receivers, " ")
	processorsStr := strings.Join(processors, " ")
	exportersStr := strings.Join(exporters, " ")

	if !isRunningAsRoot() {
		receiversStr += " run_as_user" // `inputs` is never empty, or agent will not start
	}
	if ciCompiledRegexp.MatchString(groupName) && !strings.Contains(exportersStr, "container_insights") {
		exportersStr += " container_insights"
	}

	return fmt.Sprintf("inputs:(%s) processors:(%s) outputs:(%s)", receiversStr, processorsStr, exportersStr)
}

func UserAgent(groupName string) string {
	ua, found := userAgentMap[groupName]
	if !found {
		ua = os.Getenv(envconfig.CWAGENT_USER_AGENT)
		if ua == "" {
			ua = fmt.Sprintf("%s %s", FullVersion(), Plugins(groupName))
		}
		userAgentMap[groupName] = ua
	}
	return ua
}

func FullVersion() string {
	return fmt.Sprintf("CWAgent/%s (%s; %s; %s) %s", Version(), runtime.Version(), runtime.GOOS, runtime.GOARCH, Build())
}

func SetPlugins(otelcfg *otelcol.Config, telegrafcfg *config.Config) {
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

	receivers = maps.Keys(receiverSet)
	processors = maps.Keys(processorSet)
	exporters = maps.Keys(exporterSet)

	sort.Strings(receivers)
	sort.Strings(processors)
	sort.Strings(exporters)
}

func readVersionFile() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot get the path for current executable binary: %v", err)
	}
	curPath := filepath.Dir(ex)
	versionFilePath := filepath.Join(curPath, versionFilename)
	if _, err := os.Stat(versionFilePath); err != nil {
		return "", fmt.Errorf("the agent version file %s does not exist: %v", versionFilePath, err)
	}

	byteArray, err := os.ReadFile(versionFilePath)
	if err != nil {
		return "", fmt.Errorf("issue encountered when reading content from file %s: %v", versionFilePath, err)
	}

	//TODO we may consider to do a format checking for the Version value.
	return strings.Trim(string(byteArray), " \n\r\t"), nil
}
