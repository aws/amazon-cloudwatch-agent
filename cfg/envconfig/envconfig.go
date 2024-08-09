// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package envconfig

import (
	"os"
	"runtime"
	"strconv"
	"sync"
)

const (
	//the following are the names of environment variables
	HTTP_PROXY                = "HTTP_PROXY"
	HTTPS_PROXY               = "HTTPS_PROXY"
	NO_PROXY                  = "NO_PROXY"
	AWS_CA_BUNDLE             = "AWS_CA_BUNDLE"
	AWS_SDK_LOG_LEVEL         = "AWS_SDK_LOG_LEVEL"
	CWAGENT_USER_AGENT        = "CWAGENT_USER_AGENT"
	CWAGENT_LOG_LEVEL         = "CWAGENT_LOG_LEVEL"
	CWAGENT_USAGE_DATA        = "CWAGENT_USAGE_DATA"
	IMDS_NUMBER_RETRY         = "IMDS_NUMBER_RETRY"
	RunInContainer            = "RUN_IN_CONTAINER"
	RunAsHostProcessContainer = "RUN_AS_HOST_PROCESS_CONTAINER"
	RunInAWS                  = "RUN_IN_AWS"
	RunWithIRSA               = "RUN_WITH_IRSA"
	UseDefaultConfig          = "USE_DEFAULT_CONFIG"
	HostName                  = "HOST_NAME"
	PodName                   = "POD_NAME"
	HostIP                    = "HOST_IP"
	CWConfigContent           = "CW_CONFIG_CONTENT"
)

const (
	// TrueValue is the expected string set on an environment variable to indicate true.
	TrueValue = "True"
)

var (
	usageDataEnabled bool
	onceUsageData    sync.Once
)

// getUsageDataEnabled returns true for true or invalid
// examples of invalid are not set env var, "", "invalid"
func getUsageDataEnabled() bool {
	ok, err := strconv.ParseBool(os.Getenv(CWAGENT_USAGE_DATA))
	return ok || err != nil
}

func IsUsageDataEnabled() bool {
	onceUsageData.Do(func() {
		usageDataEnabled = getUsageDataEnabled()
	})
	return usageDataEnabled
}

func IsRunningInContainer() bool {
	return os.Getenv(RunInContainer) == TrueValue
}

func IsWindowsHostProcessContainer() bool {
	if runtime.GOOS == "windows" && os.Getenv(RunInContainer) == TrueValue && os.Getenv(RunAsHostProcessContainer) == TrueValue {
		return true
	}
	return false
}
