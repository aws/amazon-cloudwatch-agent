// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	commonconfig "github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	DEFAULT_PROFILE = "AmazonCloudWatchAgent"
)

var DetectRegion = detectRegion
var DetectCredentialsPath = detectCredentialsPath
var DefaultEC2Region = defaultEC2Region
var DefaultECSRegion = defaultECSRegion
var runInAws = os.Getenv(config.RUN_IN_AWS)

func DetectAgentMode(configuredMode string) string {
	if configuredMode != "auto" {
		return configuredMode
	}

	if runInAws == config.RUN_IN_AWS_TRUE {
		fmt.Println("I! Detected from ENV instance is EC2")
		return config.ModeEC2
	}

	if DefaultEC2Region() != "" {
		fmt.Println("I! Detected the instance is EC2")
		return config.ModeEC2
	}

	if DefaultECSRegion() != "" {
		fmt.Println("I! Detected the instance is ECS")
		return config.ModeEC2
	}
	fmt.Println("I! Detected the instance is OnPremise")
	return config.ModeOnPrem
}

func SDKRegionWithCredsMap(mode string, credsConfig map[string]string) (region string) {

	credsMap := GetCredentials(mode, credsConfig)
	profile, profile_ok := credsMap[commonconfig.CredentialProfile]
	sharedConfigFile, sharedConfigFile_ok := credsMap[commonconfig.CredentialFile]
	if !profile_ok && !sharedConfigFile_ok {
		return ""
	}

	opts := session.Options{}
	if profile_ok {
		opts.Profile = profile
	}
	if sharedConfigFile_ok {
		exPath := filepath.Dir(sharedConfigFile)
		opts.SharedConfigFiles = []string{sharedConfigFile, exPath + "/config"}
	}
	CheckAndSetHomeDir()
	opts.SharedConfigState = session.SharedConfigEnable
	ses, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return ""
	}
	if ses.Config != nil && ses.Config.Region != nil {
		region = *ses.Config.Region
		fmt.Println("I! SDKRegionWithCredsMap region: ", region)
	}
	return region
}

func defaultEC2Region() string {
	return ec2util.GetEC2UtilSingleton().Region
}

func defaultECSRegion() string {
	return ecsutil.GetECSUtilSingleton().Region
}

func detectRegion(mode string, credsConfig map[string]string) (region string) {
	region = SDKRegionWithCredsMap(mode, credsConfig)

	// For ec2, fallback to metadata when no region info found in credential profile.
	if region == "" && mode == config.ModeEC2 {
		fmt.Println("I! Trying to detect region from ec2")
		region = DefaultEC2Region()
	}

	// try to get region from ecs metadata
	if region == "" && mode == config.ModeEC2 {
		fmt.Println("I! Trying to detect region from ecs")
		region = DefaultECSRegion()
	}

	return
}

func CheckAndSetHomeDir() {
	homeDir := detectHomeDirectory()
	if runtime.GOOS == config.OS_TYPE_WINDOWS {
		os.Setenv("USERPROFILE", homeDir)
		fmt.Println("I! Set home dir windows: " + homeDir)
	} else {
		os.Setenv("HOME", homeDir)
		fmt.Println("I! Set home dir Linux: " + homeDir)
	}
}

func detectCredentialsPath() (credentialsPath string) {
	homeDir := detectHomeDirectory()
	return filepath.Join(homeDir, ".aws", "credentials")
}

func detectHomeDirectory() string {
	var homeDir string
	if runtime.GOOS == config.OS_TYPE_WINDOWS {
		// the cwagent process is always running under user "System"
		systemDrivePath := GetWindowsSystemDrivePath() // C:
		homeDir = systemDrivePath + "\\Users\\Administrator"
	} else {
		if usr, err := user.Current(); err == nil {
			homeDir = usr.HomeDir
		}
		if homeDir == "" {
			if runtime.GOOS == config.OS_TYPE_DARWIN {
				homeDir = "/var/root"
			} else {
				homeDir = "/root"
			}
		}
	}
	fmt.Println("Got Home directory: " + homeDir)
	if homeDir == "" {
		translator.AddErrorMessages("/translator/util/sdkutil", "Can not get the correct Home directory")
	}

	return homeDir
}

func GetCredentials(mode string, credsConfig map[string]string) (result map[string]string) {
	result = map[string]string{}

	for k, v := range credsConfig {
		result[k] = v
	}

	profile, hasProfile := credsConfig[commonconfig.CredentialProfile]
	if hasProfile {
		result[commonconfig.CredentialProfile] = profile
	} else if (mode == config.ModeOnPrem) || (mode == config.ModeOnPremise) {
		result[commonconfig.CredentialProfile] = DEFAULT_PROFILE
	}
	return
}
