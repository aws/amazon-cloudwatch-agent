// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/eksdetector"
)

// restoreDetectionHooks snapshots the overridable detection vars and restores them on cleanup so stubs don't leak.
func restoreDetectionHooks(t *testing.T) {
	t.Helper()
	origIsEKS, origIsAKS, origIsAzureVM := IsEKS, IsAKS, IsAzureVM
	origRunInAws, origEC2, origECS := runInAws, DefaultEC2Region, DefaultECSRegion
	t.Cleanup(func() {
		IsEKS, IsAKS, IsAzureVM = origIsEKS, origIsAKS, origIsAzureVM
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
		wantMode  string
	}{
		// AWS detection must win: the Azure signals are intentionally true here and must NOT override it.
		"WithRunInAWS":  {runInAws: config.RUN_IN_AWS_TRUE, isAKS: true, isAzureVM: true, wantMode: config.ModeEC2},
		"WithEC2Region": {ec2Region: "us-east-1", isAKS: true, isAzureVM: true, wantMode: config.ModeEC2},
		"WithECSRegion": {ecsRegion: "us-east-1", isAKS: true, isAzureVM: true, wantMode: config.ModeEC2},
		// AKS nodes are Azure VMs, so the host mode resolves to AzureVM.
		"AzureHostWhenAKS":       {isAKS: true, isAzureVM: false, wantMode: config.ModeAzureVM},
		"AzureVMWhenNoAWS":       {isAzureVM: true, wantMode: config.ModeAzureVM},
		"OnPremWhenNoAWSNoAzure": {isAzureVM: false, wantMode: config.ModeOnPrem},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			runInAws = testCase.runInAws
			DefaultEC2Region = func() string { return testCase.ec2Region }
			DefaultECSRegion = func() string { return testCase.ecsRegion }
			IsAKS = func() bool { return testCase.isAKS }
			IsAzureVM = func() bool { return testCase.isAzureVM }
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
		configuredMode     string
		wantKubernetesMode string
	}{
		"EKS":           {isEKS: true, isEKSErr: nil, isAKS: false, configuredMode: config.ModeEC2, wantKubernetesMode: config.ModeEKS},
		"K8sEC2":        {isEKS: false, isEKSErr: nil, isAKS: false, configuredMode: config.ModeEC2, wantKubernetesMode: config.ModeK8sEC2},
		"K8sOnPrem":     {isEKS: false, isEKSErr: nil, isAKS: false, configuredMode: config.ModeOnPrem, wantKubernetesMode: config.ModeK8sOnPrem},
		"NotKubernetes": {isEKS: false, isEKSErr: fmt.Errorf("error"), isAKS: false, configuredMode: config.ModeEC2, wantKubernetesMode: ""},
		// RUN_IN_AKS short-circuits to AKS without the EKS probe, regardless of what EKS detection would report.
		"AKS":            {isEKS: false, isEKSErr: nil, isAKS: true, configuredMode: config.ModeAzureVM, wantKubernetesMode: config.ModeAKS},
		"AKSWhenEKSErr":  {isEKS: false, isEKSErr: fmt.Errorf("error"), isAKS: true, configuredMode: config.ModeAzureVM, wantKubernetesMode: config.ModeAKS},
		"AKSWinsOverEKS": {isEKS: true, isEKSErr: nil, isAKS: true, configuredMode: config.ModeEC2, wantKubernetesMode: config.ModeAKS},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			IsEKS = func() eksdetector.IsEKSCache {
				return eksdetector.IsEKSCache{Value: testCase.isEKS, Err: testCase.isEKSErr}
			}
			IsAKS = func() bool { return testCase.isAKS }
			require.Equal(t, testCase.wantKubernetesMode, DetectKubernetesMode(testCase.configuredMode))
		})
	}
}
