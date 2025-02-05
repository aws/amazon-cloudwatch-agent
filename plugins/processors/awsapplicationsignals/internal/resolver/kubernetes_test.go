// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/attributes"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/eksdetector"
)

// MockDeleter deletes a key immediately, useful for testing.
type MockDeleter struct{}

func (md *MockDeleter) DeleteWithDelay(m *sync.Map, key interface{}) {
	m.Delete(key)
}

var mockDeleter = &MockDeleter{}

func TestEksResolver(t *testing.T) {
	logger, _ := zap.NewProduction()
	ctx := context.Background()

	t.Run("Test getWorkloadAndNamespaceByIP", func(t *testing.T) {
		resolver := &kubernetesResolver{
			logger:                    logger,
			clusterName:               "test",
			ipToPod:                   &sync.Map{},
			podToWorkloadAndNamespace: &sync.Map{},
			ipToServiceAndNamespace:   &sync.Map{},
			serviceToWorkload:         &sync.Map{},
			useListPod:                true,
		}

		ip := "1.2.3.4"
		pod := "testPod"
		workloadAndNamespace := "testDeployment@testNamespace"

		// Pre-fill the resolver maps
		resolver.ipToPod.Store(ip, pod)
		resolver.podToWorkloadAndNamespace.Store(pod, workloadAndNamespace)

		// Test existing IP
		workload, namespace, err := resolver.getWorkloadAndNamespaceByIP(ip)
		if err != nil || workload != "testDeployment" || namespace != "testNamespace" {
			t.Errorf("Expected testDeployment@testNamespace, got %s@%s, error: %v", workload, namespace, err)
		}

		// Test non-existing IP
		_, _, err = resolver.getWorkloadAndNamespaceByIP("5.6.7.8")
		if err == nil || !strings.Contains(err.Error(), "no kubernetes workload found for ip: 5.6.7.8") {
			t.Errorf("Expected error, got %v", err)
		}

		// Test ip in ipToServiceAndNamespace but not in ipToPod
		newIP := "2.3.4.5"
		serviceAndNamespace := "testService@testNamespace"
		resolver.ipToServiceAndNamespace.Store(newIP, serviceAndNamespace)
		resolver.serviceToWorkload.Store(serviceAndNamespace, workloadAndNamespace)
		workload, namespace, err = resolver.getWorkloadAndNamespaceByIP(newIP)
		if err != nil || workload != "testDeployment" || namespace != "testNamespace" {
			t.Errorf("Expected testDeployment@testNamespace, got %s@%s, error: %v", workload, namespace, err)
		}
	})

	t.Run("Test Stop", func(t *testing.T) {
		resolver := &kubernetesResolver{
			logger:     logger,
			safeStopCh: &safeChannel{ch: make(chan struct{}), closed: false},
		}

		err := resolver.Stop(ctx)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !resolver.safeStopCh.closed {
			t.Errorf("Expected channel to be closed")
		}

		// Test closing again
		err = resolver.Stop(ctx)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Test Process", func(t *testing.T) {
		// helper function to get string values from the attributes
		getStrAttr := func(attributes pcommon.Map, key string, t *testing.T) string {
			if value, ok := attributes.Get(key); ok {
				return value.AsString()
			} else {
				t.Errorf("Failed to get value for key: %s", key)
				return ""
			}
		}

		logger, _ := zap.NewProduction()
		resolver := &kubernetesResolver{
			logger:                    logger,
			clusterName:               "test",
			platformCode:              config.PlatformEKS,
			ipToPod:                   &sync.Map{},
			podToWorkloadAndNamespace: &sync.Map{},
			ipToServiceAndNamespace:   &sync.Map{},
			serviceToWorkload:         &sync.Map{},
			useListPod:                true,
		}

		// Test case 1: "aws.remote.service" contains IP:Port
		attributes := pcommon.NewMap()
		attributes.PutStr(attr.AWSRemoteService, "192.0.2.1:8080")
		resourceAttributes := pcommon.NewMap()
		resolver.ipToPod.Store("192.0.2.1:8080", "test-pod")
		resolver.podToWorkloadAndNamespace.Store("test-pod", "test-deployment@test-namespace")
		err := resolver.Process(attributes, resourceAttributes)
		assert.NoError(t, err)
		assert.Equal(t, "test-deployment", getStrAttr(attributes, attr.AWSRemoteService, t))
		assert.Equal(t, "eks:test/test-namespace", getStrAttr(attributes, attr.AWSRemoteEnvironment, t))

		// Test case 2: "aws.remote.service" contains only IP
		attributes = pcommon.NewMap()
		attributes.PutStr(attr.AWSRemoteService, "192.0.2.2")
		resourceAttributes = pcommon.NewMap()
		resolver.ipToPod.Store("192.0.2.2", "test-pod-2")
		resolver.podToWorkloadAndNamespace.Store("test-pod-2", "test-deployment-2@test-namespace-2")
		err = resolver.Process(attributes, resourceAttributes)
		assert.NoError(t, err)
		assert.Equal(t, "test-deployment-2", getStrAttr(attributes, attr.AWSRemoteService, t))
		assert.Equal(t, "eks:test/test-namespace-2", getStrAttr(attributes, attr.AWSRemoteEnvironment, t))

		// Test case 3: "aws.remote.service" contains non-ip string
		attributes = pcommon.NewMap()
		attributes.PutStr(attr.AWSRemoteService, "not-an-ip")
		resourceAttributes = pcommon.NewMap()
		err = resolver.Process(attributes, resourceAttributes)
		assert.NoError(t, err)
		assert.Equal(t, "not-an-ip", getStrAttr(attributes, attr.AWSRemoteService, t))

		// Test case 4: Process with valid IP but getWorkloadAndNamespaceByIP returns error
		attributes = pcommon.NewMap()
		attributes.PutStr(attr.AWSRemoteService, "192.168.1.2")
		resourceAttributes = pcommon.NewMap()
		err = resolver.Process(attributes, resourceAttributes)
		assert.NoError(t, err)
		assert.Equal(t, "192.168.1.2", getStrAttr(attributes, attr.AWSRemoteService, t))
	})
}

func TestK8sResourceAttributesResolverOnEKS(t *testing.T) {
	eksdetector.NewDetector = eksdetector.TestEKSDetector
	eksdetector.IsEKS = eksdetector.TestIsEKSCacheEKS
	// helper function to get string values from the attributes
	getStrAttr := func(attributes pcommon.Map, key string, t *testing.T) string {
		if value, ok := attributes.Get(key); ok {
			return value.AsString()
		} else {
			t.Errorf("Failed to get value for key: %s", key)
			return ""
		}
	}

	resolver := newKubernetesResourceAttributesResolver(config.PlatformEKS, "test-cluster")

	resourceAttributesBase := map[string]string{
		"cloud.provider":                    "aws",
		"k8s.namespace.name":                "test-namespace-3",
		"host.id":                           "instance-id",
		"host.name":                         "hostname",
		"ec2.tag.aws:autoscaling:groupName": "asg",
	}

	tests := []struct {
		name                        string
		resourceAttributesOverwrite map[string]string
		expectedAttributes          map[string]string
	}{
		{
			"testDefault",
			map[string]string{},

			map[string]string{
				attr.AWSLocalEnvironment:            "eks:test-cluster/test-namespace-3",
				common.AttributeK8SNamespace:        "test-namespace-3",
				common.AttributeEKSClusterName:      "test-cluster",
				common.AttributeEC2InstanceId:       "instance-id",
				common.AttributeHost:                "hostname",
				common.AttributeEC2AutoScalingGroup: "asg",
			},
		},
		{
			"testOverwrite",
			map[string]string{
				semconv.AttributeDeploymentEnvironment: "custom-env",
			},
			map[string]string{
				attr.AWSLocalEnvironment:            "custom-env",
				common.AttributeK8SNamespace:        "test-namespace-3",
				common.AttributeEKSClusterName:      "test-cluster",
				common.AttributeEC2InstanceId:       "instance-id",
				common.AttributeHost:                "hostname",
				common.AttributeEC2AutoScalingGroup: "asg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attributes := pcommon.NewMap()
			resourceAttributes := pcommon.NewMap()
			for key, val := range resourceAttributesBase {
				resourceAttributes.PutStr(key, val)
			}
			for key, val := range tt.resourceAttributesOverwrite {
				resourceAttributes.PutStr(key, val)
			}
			err := resolver.Process(attributes, resourceAttributes)
			assert.NoError(t, err)

			for key, val := range tt.expectedAttributes {
				assert.Equal(t, val, getStrAttr(attributes, key, t), fmt.Sprintf("expected %s for key %s", val, key))
			}
			assert.Equal(t, "/aws/containerinsights/test-cluster/application", getStrAttr(resourceAttributes, semconv.AttributeAWSLogGroupNames, t))
		})
	}
}

func TestK8sResourceAttributesResolverOnK8S(t *testing.T) {
	eksdetector.NewDetector = eksdetector.TestK8sDetector
	eksdetector.IsEKS = eksdetector.TestIsEKSCacheK8s
	// helper function to get string values from the attributes
	getStrAttr := func(attributes pcommon.Map, key string, t *testing.T) string {
		if value, ok := attributes.Get(key); ok {
			return value.AsString()
		} else {
			t.Errorf("Failed to get value for key: %s", key)
			return ""
		}
	}

	resolver := newKubernetesResourceAttributesResolver(config.PlatformK8s, "test-cluster")

	resourceAttributesBase := map[string]string{
		"cloud.provider":                    "aws",
		"k8s.namespace.name":                "test-namespace-3",
		"host.id":                           "instance-id",
		"host.name":                         "hostname",
		"ec2.tag.aws:autoscaling:groupName": "asg",
	}

	tests := []struct {
		name                        string
		resourceAttributesOverwrite map[string]string
		expectedAttributes          map[string]string
	}{
		{
			"testDefaultOnK8s",
			map[string]string{},

			map[string]string{
				attr.AWSLocalEnvironment:            "k8s:test-cluster/test-namespace-3",
				common.AttributeK8SNamespace:        "test-namespace-3",
				common.AttributeK8SClusterName:      "test-cluster",
				common.AttributeEC2InstanceId:       "instance-id",
				common.AttributeHost:                "hostname",
				common.AttributeEC2AutoScalingGroup: "asg",
			},
		},
		{
			"testOverwriteOnK8s",
			map[string]string{
				semconv.AttributeDeploymentEnvironment: "custom-env",
			},
			map[string]string{
				attr.AWSLocalEnvironment:            "custom-env",
				common.AttributeK8SNamespace:        "test-namespace-3",
				common.AttributeK8SClusterName:      "test-cluster",
				common.AttributeEC2InstanceId:       "instance-id",
				common.AttributeHost:                "hostname",
				common.AttributeEC2AutoScalingGroup: "asg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attributes := pcommon.NewMap()
			resourceAttributes := pcommon.NewMap()
			for key, val := range resourceAttributesBase {
				resourceAttributes.PutStr(key, val)
			}
			for key, val := range tt.resourceAttributesOverwrite {
				resourceAttributes.PutStr(key, val)
			}
			err := resolver.Process(attributes, resourceAttributes)
			assert.NoError(t, err)

			for key, val := range tt.expectedAttributes {
				assert.Equal(t, val, getStrAttr(attributes, key, t), fmt.Sprintf("expected %s for key %s", val, key))
			}
			assert.Equal(t, "/aws/containerinsights/test-cluster/application", getStrAttr(resourceAttributes, semconv.AttributeAWSLogGroupNames, t))
		})
	}
}

func TestK8sResourceAttributesResolverOnK8SOnPrem(t *testing.T) {
	eksdetector.NewDetector = eksdetector.TestK8sDetector
	// helper function to get string values from the attributes
	getStrAttr := func(attributes pcommon.Map, key string, t *testing.T) string {
		if value, ok := attributes.Get(key); ok {
			return value.AsString()
		} else {
			t.Errorf("Failed to get value for key: %s", key)
			return ""
		}
	}

	resolver := newKubernetesResourceAttributesResolver(config.PlatformK8s, "test-cluster")

	resourceAttributesBase := map[string]string{
		"cloud.provider":     "aws",
		"k8s.namespace.name": "test-namespace-3",
		"host.name":          "hostname",
	}

	tests := []struct {
		name                        string
		resourceAttributesOverwrite map[string]string
		expectedAttributes          map[string]string
	}{
		{
			"testDefault",
			map[string]string{},

			map[string]string{
				attr.AWSLocalEnvironment:       "k8s:test-cluster/test-namespace-3",
				common.AttributeK8SNamespace:   "test-namespace-3",
				common.AttributeK8SClusterName: "test-cluster",
				common.AttributeHost:           "hostname",
			},
		},
		{
			"testOverwrite",
			map[string]string{
				semconv.AttributeDeploymentEnvironment: "custom-env",
			},
			map[string]string{
				attr.AWSLocalEnvironment:       "custom-env",
				common.AttributeK8SNamespace:   "test-namespace-3",
				common.AttributeK8SClusterName: "test-cluster",
				common.AttributeHost:           "hostname",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attributes := pcommon.NewMap()
			resourceAttributes := pcommon.NewMap()
			for key, val := range resourceAttributesBase {
				resourceAttributes.PutStr(key, val)
			}
			for key, val := range tt.resourceAttributesOverwrite {
				resourceAttributes.PutStr(key, val)
			}
			err := resolver.Process(attributes, resourceAttributes)
			assert.NoError(t, err)

			for key, val := range tt.expectedAttributes {
				assert.Equal(t, val, getStrAttr(attributes, key, t), fmt.Sprintf("expected %s for key %s", val, key))
			}
			assert.Equal(t, "/aws/containerinsights/test-cluster/application", getStrAttr(resourceAttributes, semconv.AttributeAWSLogGroupNames, t))

			// EC2 related fields that should not exist for on-prem
			_, exists := attributes.Get(common.AttributeEC2AutoScalingGroup)
			assert.False(t, exists)

			_, exists = attributes.Get(common.AttributeEC2InstanceId)
			assert.False(t, exists)
		})
	}
}
