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

func TestDetectAgentModeAuto(t *testing.T) {
	testCases := map[string]struct {
		runInAws  string
		ec2Region string
		ecsRegion string
		wantMode  string
	}{
		"WithRunInAWS":  {runInAws: config.RUN_IN_AWS_TRUE, wantMode: config.ModeEC2},
		"WithEC2Region": {ec2Region: "us-east-1", wantMode: config.ModeEC2},
		"WithECSRegion": {ecsRegion: "us-east-1", wantMode: config.ModeEC2},
		"WithNone":      {wantMode: config.ModeOnPrem},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			runInAws = testCase.runInAws
			DefaultEC2Region = func() string { return testCase.ec2Region }
			DefaultECSRegion = func() string { return testCase.ecsRegion }
			require.Equal(t, testCase.wantMode, DetectAgentMode("auto"))
		})
	}
}

func TestDetectKubernetesMode(t *testing.T) {
	testCases := map[string]struct {
		isEKS              bool
		isEKSErr           error
		configuredMode     string
		wantKubernetesMode string
	}{
		"EKS":           {isEKS: true, isEKSErr: nil, configuredMode: config.ModeEC2, wantKubernetesMode: config.ModeEKS},
		"K8sEC2":        {isEKS: false, isEKSErr: nil, configuredMode: config.ModeEC2, wantKubernetesMode: config.ModeK8sEC2},
		"K8sOnPrem":     {isEKS: false, isEKSErr: nil, configuredMode: config.ModeOnPrem, wantKubernetesMode: config.ModeK8sOnPrem},
		"NotKubernetes": {isEKS: false, isEKSErr: fmt.Errorf("error"), configuredMode: config.ModeEC2, wantKubernetesMode: ""},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			IsEKS = func() eksdetector.IsEKSCache {
				return eksdetector.IsEKSCache{Value: testCase.isEKS, Err: testCase.isEKSErr}
			}
			require.Equal(t, testCase.wantKubernetesMode, DetectKubernetesMode(testCase.configuredMode))
		})
	}
}
