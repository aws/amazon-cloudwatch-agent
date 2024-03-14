// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/eksdetector"
)

const (
	DEFAULT_PROFILE = "AmazonCloudWatchAgent"
)

var DetectRegion = detectRegion
var DetectCredentialsPath = detectCredentialsPath
var DefaultEC2Region = defaultEC2Region
var DefaultECSRegion = defaultECSRegion
var IsEKS = isEKS
var runInAws = os.Getenv(config.RUN_IN_AWS)
var runWithIrsa = os.Getenv(config.RUN_WITH_IRSA)

func DetectAgentMode(configuredMode string) string {
	if configuredMode != "auto" {
		return configuredMode
	}

	if runInAws == config.RUN_IN_AWS_TRUE {
		fmt.Println("I! Detected from ENV instance is EC2")
		return config.ModeEC2
	}

	if runWithIrsa == config.RUN_WITH_IRSA_TRUE {
		fmt.Println("I! Detected from ENV RUN_WITH_IRSA is True")
		return config.ModeWithIRSA
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

func DetectKubernetesMode(configuredMode string) string {
	isEKS := IsEKS()

	if isEKS.Err != nil {
		return "" // not kubernetes
	}

	if isEKS.Value {
		return config.ModeEKS
	}

	if configuredMode == config.ModeEC2 {
		return config.ModeK8sEC2
	}

	return config.ModeK8sOnPrem

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

func isEKS() eksdetector.IsEKSCache {
	return eksdetector.IsEKS()
}

func detectRegion(mode string, credsConfig map[string]string) (region string, regionType string) {
	region = SDKRegionWithCredsMap(mode, credsConfig)
	regionType = config.RegionTypeNotFound
	if region != "" {
		regionType = config.RegionTypeCredsMap
	}

	// For ec2, fallback to metadata when no region info found in credential profile.
	if region == "" && mode == config.ModeEC2 {

		fmt.Println("I! Trying to detect region from ec2")
		region = DefaultEC2Region()
		regionType = config.RegionTypeEC2Metadata
	}

	// try to get region from ecs metadata
	if region == "" && mode == config.ModeEC2 {
		fmt.Println("I! Trying to detect region from ecs")
		region = DefaultECSRegion()
		regionType = config.RegionTypeECSMetadata
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
