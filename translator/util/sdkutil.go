// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/azuredetector"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/eksdetector"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/gcpdetector"
)

const (
	DefaultProfile = "AmazonCloudWatchAgent"
)

var DetectRegion = detectRegion
var DetectCredentialsPath = detectCredentialsPath
var DefaultEC2Region = defaultEC2Region
var DefaultECSRegion = defaultECSRegion
var IsEKS = isEKS

// IsAKS/IsAzureVM are overridable detection hooks; tests override these vars.
var IsAKS = azuredetector.IsAKS
var IsAzureVM = azuredetector.IsAzureVM

// IsGCE/IsGKE are overridable detection hooks; tests override these vars.
var IsGCE = gcpdetector.IsGCE
var IsGKE = gcpdetector.IsGKE
var runInAws = os.Getenv(translatorconfig.RUN_IN_AWS)
var runWithIrsa = os.Getenv(translatorconfig.RUN_WITH_IRSA)

func DetectAgentMode(configuredMode string) string {
	if configuredMode != "auto" {
		return configuredMode
	}

	if runInAws == translatorconfig.RUN_IN_AWS_TRUE {
		fmt.Println("I! Detected from ENV instance is EC2")
		return translatorconfig.ModeEC2
	}

	if runWithIrsa == translatorconfig.RUN_WITH_IRSA_TRUE {
		fmt.Println("I! Detected from ENV RUN_WITH_IRSA is True")
		return translatorconfig.ModeWithIRSA
	}

	if DefaultEC2Region() != "" {
		fmt.Println("I! Detected the instance is EC2")
		return translatorconfig.ModeEC2
	}

	if DefaultECSRegion() != "" {
		fmt.Println("I! Detected the instance is ECS")
		return translatorconfig.ModeEC2
	}

	// Azure is checked only after all AWS signals; RUN_IN_AKS is checked before the IMDS probe since an AKS pod may not reach IMDS.
	if IsAKS() {
		fmt.Println("I! Detected from ENV instance is Azure (AKS)")
		return translatorconfig.ModeAzureVM
	}

	// Last resort for Azure: IMDS probe (~2s worst-case one-time cost on a black-holed host).
	if IsAzureVM() {
		fmt.Println("I! Detected the instance is Azure VM")
		return translatorconfig.ModeAzureVM
	}

	// GCP is checked after Azure: metadata-server probe (cached by the SDK).
	if IsGCE() {
		fmt.Println("I! Detected the instance is GCE")
		return translatorconfig.ModeGCE
	}

	fmt.Println("I! Detected the instance is OnPremise")
	return translatorconfig.ModeOnPrem
}

// DetectECS reports whether the agent is running on ECS. It indirects the
// ecsutil singleton so callers don't depend on it directly, mirroring
// DetectKubernetesMode.
func DetectECS() bool {
	return ecsutil.GetECSUtilSingleton().IsECS()
}

func DetectKubernetesMode(configuredMode string) string {
	// RUN_IN_AKS is an explicit env signal (no I/O), so short-circuit before the EKS in-cluster probe.
	if IsAKS() {
		return translatorconfig.ModeAKS
	}

	// RUN_IN_GKE is likewise an explicit env signal (no I/O).
	if IsGKE() {
		return translatorconfig.ModeGKE
	}

	isEKS := IsEKS()
	if isEKS.Err == nil && isEKS.Value {
		return translatorconfig.ModeEKS
	}

	if isEKS.Err != nil {
		return "" // not kubernetes
	}

	if configuredMode == translatorconfig.ModeEC2 {
		return translatorconfig.ModeK8sEC2
	}

	return translatorconfig.ModeK8sOnPrem

}

func SDKRegionWithCredsMap(mode string, credsConfig map[string]string) string {
	credsMap := GetCredentials(mode, credsConfig)
	profile, profileOK := credsMap[commonconfig.CredentialProfile]
	sharedConfigFile, sharedConfigFileOK := credsMap[commonconfig.CredentialFile]
	if !profileOK && !sharedConfigFileOK {
		return ""
	}

	CheckAndSetHomeDir()

	var opts []func(*config.LoadOptions) error
	if profileOK {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if sharedConfigFileOK {
		exPath := filepath.Dir(sharedConfigFile)
		opts = append(opts, config.WithSharedConfigFiles([]string{sharedConfigFile, filepath.Join(exPath, "config")}))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return ""
	}

	region := cfg.Region
	if region != "" {
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

func detectRegion(mode string, credsConfig map[string]string) (string, string) {
	region := SDKRegionWithCredsMap(mode, credsConfig)
	regionType := translatorconfig.RegionTypeNotFound
	if region != "" {
		regionType = translatorconfig.RegionTypeCredsMap
	}

	// For ec2, fallback to metadata when no region info found in credential profile.
	if region == "" && mode == translatorconfig.ModeEC2 {

		fmt.Println("I! Trying to detect region from ec2")
		region = util.DefaultEC2Region(context.Background())
		regionType = translatorconfig.RegionTypeEC2Metadata
	}

	// try to get region from ecs metadata
	if region == "" && mode == translatorconfig.ModeEC2 {
		fmt.Println("I! Trying to detect region from ecs")
		region = DefaultECSRegion()
		regionType = translatorconfig.RegionTypeECSMetadata
	}

	return region, regionType
}

func CheckAndSetHomeDir() {
	homeDir := detectHomeDirectory()
	if runtime.GOOS == translatorconfig.OS_TYPE_WINDOWS {
		os.Setenv("USERPROFILE", homeDir)
		fmt.Println("I! Set home dir windows: " + homeDir)
	} else {
		os.Setenv("HOME", homeDir)
		fmt.Println("I! Set home dir Linux: " + homeDir)
	}
}

func detectCredentialsPath() string {
	homeDir := detectHomeDirectory()
	return filepath.Join(homeDir, ".aws", "credentials")
}

func detectHomeDirectory() string {
	var homeDir string
	if runtime.GOOS == translatorconfig.OS_TYPE_WINDOWS {
		// the cwagent process is always running under user "System"
		systemDrivePath := GetWindowsSystemDrivePath() // C:
		homeDir = systemDrivePath + "\\Users\\Administrator"
	} else {
		if usr, err := user.Current(); err == nil {
			homeDir = usr.HomeDir
		}
		if homeDir == "" {
			if runtime.GOOS == translatorconfig.OS_TYPE_DARWIN {
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

func GetCredentials(mode string, credsConfig map[string]string) map[string]string {
	result := map[string]string{}

	for k, v := range credsConfig {
		result[k] = v
	}

	profile, hasProfile := credsConfig[commonconfig.CredentialProfile]
	if hasProfile {
		result[commonconfig.CredentialProfile] = profile
	} else if (mode == translatorconfig.ModeOnPrem) || (mode == translatorconfig.ModeOnPremise) {
		result[commonconfig.CredentialProfile] = DefaultProfile
	}
	return result
}
