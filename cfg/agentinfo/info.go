// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentinfo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
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
	VersionStr    string
	BuildStr      string = "No Build Date"
	InputPlugins  []string
	OutputPlugins []string

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
	outputs := strings.Join(OutputPlugins, " ")
	inputs := strings.Join(InputPlugins, " ")

	if !isRunningAsRoot() {
		inputs += " run_as_user" // `inputs` is never empty, or agent will not start
	}
	if ciCompiledRegexp.MatchString(groupName) && !strings.Contains(outputs, "container_insights") {
		outputs += " container_insights"
	}

	return fmt.Sprintf("inputs:(%s) outputs:(%s)", inputs, outputs)
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

	byteArray, err := ioutil.ReadFile(versionFilePath)
	if err != nil {
		return "", fmt.Errorf("issue encountered when reading content from file %s: %v", versionFilePath, err)
	}

	//TODO we may consider to do a format checking for the Version value.
	return strings.Trim(string(byteArray), " \n\r\t"), nil
}
