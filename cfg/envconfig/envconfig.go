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
	HTTP_PROXY                  = "HTTP_PROXY"         //nolint:revive
	HTTPS_PROXY                 = "HTTPS_PROXY"        //nolint:revive
	NO_PROXY                    = "NO_PROXY"           //nolint:revive
	AWS_CA_BUNDLE               = "AWS_CA_BUNDLE"      //nolint:revive
	AWS_SDK_LOG_LEVEL           = "AWS_SDK_LOG_LEVEL"  //nolint:revive
	CWAGENT_USER_AGENT          = "CWAGENT_USER_AGENT" //nolint:revive
	CWAGENT_LOG_LEVEL           = "CWAGENT_LOG_LEVEL"  //nolint:revive
	CWAGENT_ROLE                = "CWAGENT_ROLE"       //nolint:revive
	CWAGENT_USAGE_DATA          = "CWAGENT_USAGE_DATA" //nolint:revive
	IMDS_NUMBER_RETRY           = "IMDS_NUMBER_RETRY"  //nolint:revive
	RunInContainer              = "RUN_IN_CONTAINER"
	RunAsHostProcessContainer   = "RUN_AS_HOST_PROCESS_CONTAINER"
	RunInAWS                    = "RUN_IN_AWS"
	RunWithIRSA                 = "RUN_WITH_IRSA"
	RunWithSELinux              = "RUN_WITH_SELINUX"
	RunInROSA                   = "RUN_IN_ROSA"
	UseDefaultConfig            = "USE_DEFAULT_CONFIG"
	HostName                    = "HOST_NAME"
	PodName                     = "POD_NAME"
	HostIP                      = "HOST_IP"
	CWConfigContent             = "CW_CONFIG_CONTENT"
	CWOtelConfigContent         = "CW_OTEL_CONFIG_CONTENT"
	CWAgentMergedOtelConfig     = "CWAGENT_MERGED_OTEL_CONFIG"
	CWAgentLogsBackpressureMode = "CWAGENT_LOGS_BACKPRESSURE_MODE"

	// confused deputy prevention related headers
	AmzSourceAccount = "AMZ_SOURCE_ACCOUNT" // populates the "x-amz-source-account" header
	AmzSourceArn     = "AMZ_SOURCE_ARN"     // populates the "x-amz-source-arn" header
)

const (
	TrueValue = "True"   // TrueValue is the expected string set on an environment variable to indicate true.
	LEADER    = "LEADER" //nolint:revive
	NODE      = "NODE"   //nolint:revive
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

func IsSelinuxEnabled() bool {
	return os.Getenv(RunWithSELinux) == TrueValue
}

func IsRunningInROSA() bool {
	return os.Getenv(RunInROSA) == TrueValue
}

func GetLogsBackpressureMode() string {
	return os.Getenv(CWAgentLogsBackpressureMode)
}
