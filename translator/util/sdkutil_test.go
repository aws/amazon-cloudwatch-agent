// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata"
	"github.com/aws/amazon-cloudwatch-agent/internal/cloudprovider"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/eksdetector"
)

type mockProvider struct {
	region        string
	cloudProvider cloudprovider.CloudProvider
}

func (m *mockProvider) Region() string                             { return m.region }
func (m *mockProvider) InstanceID() string                         { return "i-test" }
func (m *mockProvider) Hostname() string                           { return "test" }
func (m *mockProvider) InstanceType() string                       { return "t3.micro" }
func (m *mockProvider) ImageID() string                            { return "" }
func (m *mockProvider) AccountID() string                          { return "123456" }
func (m *mockProvider) PrivateIP() string                          { return "10.0.0.1" }
func (m *mockProvider) CloudProvider() cloudprovider.CloudProvider { return m.cloudProvider }

func TestDetectAgentModeAuto(t *testing.T) {
	testCases := map[string]struct {
		runInAws  string
		provider  cloudmetadata.Provider
		ecsRegion string
		wantMode  string
	}{
		"WithRunInAWS":  {runInAws: config.RUN_IN_AWS_TRUE, wantMode: config.ModeEC2},
		"WithEC2Region": {provider: &mockProvider{region: "us-east-1", cloudProvider: cloudprovider.AWS}, wantMode: config.ModeEC2},
		"WithAzure":     {provider: &mockProvider{region: "westus2", cloudProvider: cloudprovider.Azure}, wantMode: config.ModeOnPrem},
		"WithECSRegion": {ecsRegion: "us-east-1", wantMode: config.ModeEC2},
		"WithNone":      {wantMode: config.ModeOnPrem},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			runInAws = testCase.runInAws
			cloudmetadata.SetForTest(testCase.provider)
			DefaultECSRegion = func() string { return testCase.ecsRegion }
			require.Equal(t, testCase.wantMode, DetectAgentMode("auto"))
			runInAws = ""
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
