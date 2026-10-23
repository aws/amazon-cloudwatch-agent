// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/eksdetector"
)

// restoreDetectionHooks snapshots the overridable detection vars and restores them on cleanup so stubs don't leak.
func restoreDetectionHooks(t *testing.T) {
	t.Helper()
	origIsEKS, origIsAKS, origIsAzureVM := IsEKS, IsAKS, IsAzureVM
	origIsGCE, origIsGKE := IsGCE, IsGKE
	origRunInAws, origEC2, origECS := runInAws, DefaultEC2Region, DefaultECSRegion
	t.Cleanup(func() {
		IsEKS, IsAKS, IsAzureVM = origIsEKS, origIsAKS, origIsAzureVM
		IsGCE, IsGKE = origIsGCE, origIsGKE
		runInAws, DefaultEC2Region, DefaultECSRegion = origRunInAws, origEC2, origECS
	})
}

func TestDetectAgentModeAuto(t *testing.T) {
	restoreDetectionHooks(t)

	testCases := map[string]struct {
		runInAws  string
		ec2Region string
		ecsRegion string
		isAKS     bool
		isAzureVM bool
		isGCE     bool
		wantMode  string
	}{
		// AWS detection must win: the Azure/GCP signals are intentionally true here and must NOT override it.
		"WithRunInAWS":  {runInAws: config.RUN_IN_AWS_TRUE, isAKS: true, isAzureVM: true, isGCE: true, wantMode: config.ModeEC2},
		"WithEC2Region": {ec2Region: "us-east-1", isAKS: true, isAzureVM: true, isGCE: true, wantMode: config.ModeEC2},
		"WithECSRegion": {ecsRegion: "us-east-1", isAKS: true, isAzureVM: true, isGCE: true, wantMode: config.ModeEC2},
		// AKS nodes are Azure VMs, so the host mode resolves to AzureVM.
		"AzureHostWhenAKS": {isAKS: true, isAzureVM: false, wantMode: config.ModeAzureVM},
		"AzureVMWhenNoAWS": {isAzureVM: true, wantMode: config.ModeAzureVM},
		"GCEWhenNoAWS":     {isGCE: true, wantMode: config.ModeGCE},
		// Azure is probed before GCP, so it wins when both report true.
		"AzureWinsOverGCP":       {isAzureVM: true, isGCE: true, wantMode: config.ModeAzureVM},
		"OnPremWhenNoAWSNoCloud": {isAzureVM: false, isGCE: false, wantMode: config.ModeOnPrem},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			runInAws = testCase.runInAws
			DefaultEC2Region = func() string { return testCase.ec2Region }
			DefaultECSRegion = func() string { return testCase.ecsRegion }
			IsAKS = func() bool { return testCase.isAKS }
			IsAzureVM = func() bool { return testCase.isAzureVM }
			IsGCE = func() bool { return testCase.isGCE }
			require.Equal(t, testCase.wantMode, DetectAgentMode("auto"))
		})
	}
}

func TestDetectKubernetesMode(t *testing.T) {
	restoreDetectionHooks(t)

	testCases := map[string]struct {
		isEKS              bool
		isEKSErr           error
		isAKS              bool
		isGKE              bool
		configuredMode     string
		wantKubernetesMode string
	}{
		"EKS":           {isEKS: true, isEKSErr: nil, configuredMode: config.ModeEC2, wantKubernetesMode: config.ModeEKS},
		"K8sEC2":        {isEKS: false, isEKSErr: nil, configuredMode: config.ModeEC2, wantKubernetesMode: config.ModeK8sEC2},
		"K8sOnPrem":     {isEKS: false, isEKSErr: nil, configuredMode: config.ModeOnPrem, wantKubernetesMode: config.ModeK8sOnPrem},
		"NotKubernetes": {isEKS: false, isEKSErr: fmt.Errorf("error"), configuredMode: config.ModeEC2, wantKubernetesMode: ""},
		// RUN_IN_AKS short-circuits to AKS without the EKS probe, regardless of what EKS detection would report.
		"AKS":            {isEKS: false, isEKSErr: nil, isAKS: true, configuredMode: config.ModeAzureVM, wantKubernetesMode: config.ModeAKS},
		"AKSWhenEKSErr":  {isEKS: false, isEKSErr: fmt.Errorf("error"), isAKS: true, configuredMode: config.ModeAzureVM, wantKubernetesMode: config.ModeAKS},
		"AKSWinsOverEKS": {isEKS: true, isEKSErr: nil, isAKS: true, configuredMode: config.ModeEC2, wantKubernetesMode: config.ModeAKS},
		// RUN_IN_GKE likewise short-circuits to GKE without the EKS probe.
		"GKE":            {isEKS: false, isEKSErr: nil, isGKE: true, configuredMode: config.ModeGCE, wantKubernetesMode: config.ModeGKE},
		"GKEWhenEKSErr":  {isEKS: false, isEKSErr: fmt.Errorf("error"), isGKE: true, configuredMode: config.ModeGCE, wantKubernetesMode: config.ModeGKE},
		"GKEWinsOverEKS": {isEKS: true, isEKSErr: nil, isGKE: true, configuredMode: config.ModeEC2, wantKubernetesMode: config.ModeGKE},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			IsEKS = func() eksdetector.IsEKSCache {
				return eksdetector.IsEKSCache{Value: testCase.isEKS, Err: testCase.isEKSErr}
			}
			IsAKS = func() bool { return testCase.isAKS }
			IsGKE = func() bool { return testCase.isGKE }
			require.Equal(t, testCase.wantKubernetesMode, DetectKubernetesMode(testCase.configuredMode))
		})
	}
}

// TestSDKRegionWithCredsMap exercises the real SDK-backed lookup (no DetectRegion stub) for what
// this function adds around configaws.SharedConfigRegion: the early return without credentials
// config, the default profile forced for onPrem, and the configured credentials file being read in
// the credentials slot with its sibling "config" in the config slot.
func TestSDKRegionWithCredsMap(t *testing.T) {
	const (
		namedCreds   = "[" + DefaultProfile + "]\nregion = eu-central-1\n"
		defaultCreds = "[default]\nregion = us-west-1\n"
		namedConfig  = "[profile " + DefaultProfile + "]\nregion = ap-south-1\n"
	)
	testCases := map[string]struct {
		env         map[string]string
		credsFile   string // content of <dir>/credentials; "" for none
		configFile  string // content of <dir>/config; "" for none
		credsConfig map[string]string
		wantOnPrem  string
		wantEC2     string
	}{
		// onPrem forces the default profile so the lookup runs; other modes return early.
		"EnvOnly/NoCredsConfig": {env: map[string]string{"AWS_REGION": "eu-west-1"}, wantOnPrem: "eu-west-1", wantEC2: ""},
		"EnvOnly/Profile":       {env: map[string]string{"AWS_REGION": "eu-west-1"}, credsConfig: map[string]string{commonconfig.CredentialProfile: DefaultProfile}, wantOnPrem: "eu-west-1", wantEC2: "eu-west-1"},
		// Without a profile, ec2 reads [default].
		"CredentialsFile/NamedSection": {credsFile: namedCreds + defaultCreds, credsConfig: map[string]string{commonconfig.CredentialFile: "credentials"}, wantOnPrem: "eu-central-1", wantEC2: "us-west-1"},
		"CredentialsFile/NamedProfile": {credsFile: namedCreds, credsConfig: map[string]string{commonconfig.CredentialProfile: DefaultProfile, commonconfig.CredentialFile: "credentials"}, wantOnPrem: "eu-central-1", wantEC2: "eu-central-1"},
		"ConfigFile/NamedProfile":      {configFile: namedConfig, credsConfig: map[string]string{commonconfig.CredentialProfile: DefaultProfile, commonconfig.CredentialFile: "credentials"}, wantOnPrem: "ap-south-1", wantEC2: "ap-south-1"},
		"Nothing":                      {credsConfig: map[string]string{commonconfig.CredentialProfile: DefaultProfile, commonconfig.CredentialFile: "credentials"}, wantOnPrem: "", wantEC2: ""},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := testutil.IsolateAWSSharedConfigEnv(t)
			for k, v := range testCase.env {
				t.Setenv(k, v)
			}
			if testCase.credsFile != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "credentials"), []byte(testCase.credsFile), 0o600))
			}
			if testCase.configFile != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "config"), []byte(testCase.configFile), 0o600))
			}
			credsConfig := map[string]string{}
			for k, v := range testCase.credsConfig {
				if k == commonconfig.CredentialFile {
					v = filepath.Join(dir, v)
				}
				credsConfig[k] = v
			}
			assert.Equal(t, testCase.wantOnPrem, SDKRegionWithCredsMap(config.ModeOnPrem, credsConfig), "onPrem")
			assert.Equal(t, testCase.wantEC2, SDKRegionWithCredsMap(config.ModeEC2, credsConfig), "ec2")
		})
	}
}

// TestDetectRegion covers the fallback order around the SDK lookup: a region from the credentials
// map wins over metadata, and only ec2 falls back to EC2 then ECS metadata.
func TestDetectRegion(t *testing.T) {
	restoreDetectionHooks(t)
	DefaultECSRegion = func() string { return "us-east-2" }

	testCases := map[string]struct {
		mode       string
		credsFile  string
		ec2Region  string
		wantRegion string
		wantType   string
	}{
		"CredsMapBeatsMetadata/EC2": {mode: config.ModeEC2, credsFile: "[" + DefaultProfile + "]\nregion = eu-central-1\n", ec2Region: "us-east-1", wantRegion: "eu-central-1", wantType: config.RegionTypeCredsMap},
		"EC2Metadata/EC2":           {mode: config.ModeEC2, ec2Region: "us-east-1", wantRegion: "us-east-1", wantType: config.RegionTypeEC2Metadata},
		"ECSMetadata/EC2":           {mode: config.ModeEC2, wantRegion: "us-east-2", wantType: config.RegionTypeECSMetadata},
		// A profile the SDK cannot parse yields no region from the credentials map.
		"BrokenProfile/EC2":         {mode: config.ModeEC2, credsFile: "[" + DefaultProfile + "]\naws_access_key_id = AKIATEST\nregion = eu-central-1\n", ec2Region: "us-east-1", wantRegion: "us-east-1", wantType: config.RegionTypeEC2Metadata},
		"NoMetadataFallback/OnPrem": {mode: config.ModeOnPrem, ec2Region: "us-east-1", wantRegion: "", wantType: config.RegionTypeNotFound},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := testutil.IsolateAWSSharedConfigEnv(t)
			DefaultEC2Region = func() string { return testCase.ec2Region }
			credsConfig := map[string]string{}
			if testCase.credsFile != "" {
				path := filepath.Join(dir, "credentials")
				require.NoError(t, os.WriteFile(path, []byte(testCase.credsFile), 0o600))
				credsConfig = map[string]string{commonconfig.CredentialProfile: DefaultProfile, commonconfig.CredentialFile: path}
			}
			region, regionType := detectRegion(testCase.mode, credsConfig)
			assert.Equal(t, testCase.wantRegion, region)
			assert.Equal(t, testCase.wantType, regionType)
		})
	}
}
