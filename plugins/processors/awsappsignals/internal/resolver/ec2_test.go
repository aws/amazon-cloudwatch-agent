// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/internal/attributes"
)

func TestEC2HostedInAttributeResolverWithNoConfiguredName_NoASG_NoEnv(t *testing.T) {
	resolver := newEC2HostedInAttributeResolver("")

	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEC2Environment)
	assert.True(t, ok)
	assert.Equal(t, AttributePlatformEC2, envAttr.AsString())
}

func TestEC2HostedInAttributeResolverWithNoConfiguredName_ASGExists_NoEnv(t *testing.T) {
	resolver := newEC2HostedInAttributeResolver("")

	asgName := "ASG"
	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()
	resourceAttributes.PutStr(attr.ResourceDetectionASG, asgName)

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEC2Environment)
	assert.True(t, ok)
	assert.Equal(t, asgName, envAttr.AsString())
}

func TestEC2HostedInAttributeResolverWithConfiguredName_NoASG_NoEnv(t *testing.T) {
	resolver := newEC2HostedInAttributeResolver("test")

	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEC2Environment)
	assert.True(t, ok)
	assert.Equal(t, "test", envAttr.AsString())
}

func TestEC2HostedInAttributeResolverWithConfiguredName_ASGExists_NoEnv(t *testing.T) {
	resolver := newEC2HostedInAttributeResolver("test")

	asgName := "ASG"
	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()
	resourceAttributes.PutStr(attr.ResourceDetectionASG, asgName)

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEC2Environment)
	assert.True(t, ok)
	assert.Equal(t, asgName, envAttr.AsString())
}

func TestEC2HostedInAttributeResolverWithNoConfiguredName_NoASG_EnvExists(t *testing.T) {
	resolver := newEC2HostedInAttributeResolver("")

	envName := "my-env"
	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()
	resourceAttributes.PutStr(attr.AWSHostedInEnvironment, envName)

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEC2Environment)
	assert.True(t, ok)
	assert.Equal(t, envName, envAttr.AsString())
}

func TestEC2HostedInAttributeResolverWithConfiguredName_NoASG_EnvExists(t *testing.T) {
	resolver := newEC2HostedInAttributeResolver("test")

	envName := "my-env"
	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()
	resourceAttributes.PutStr(attr.AWSHostedInEnvironment, envName)

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEC2Environment)
	assert.True(t, ok)
	assert.Equal(t, envName, envAttr.AsString())
}

func TestEC2HostedInAttributeResolverWithNoConfiguredName_ASGExists_EnvExists(t *testing.T) {
	resolver := newEC2HostedInAttributeResolver("")

	asgName := "ASG"
	envName := "my-env"
	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()
	resourceAttributes.PutStr(attr.EC2AutoScalingGroupName, asgName)
	resourceAttributes.PutStr(attr.AWSHostedInEnvironment, envName)

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEC2Environment)
	assert.True(t, ok)
	assert.Equal(t, envName, envAttr.AsString())
}

func TestEC2HostedInAttributeResolverWithConfiguredName_ASGExists_EnvExists(t *testing.T) {
	resolver := newEC2HostedInAttributeResolver("test")

	asgName := "ASG"
	envName := "my-env"
	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()
	resourceAttributes.PutStr(attr.EC2AutoScalingGroupName, asgName)
	resourceAttributes.PutStr(attr.AWSHostedInEnvironment, envName)

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEC2Environment)
	assert.True(t, ok)
	assert.Equal(t, envName, envAttr.AsString())
}

func TestEC2HostedInAttributeResolverWithResourceDetectionAttributes(t *testing.T) {
	resolver := newEC2HostedInAttributeResolver("")

	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()
	resourceAttributes.PutStr(attr.ResourceDetectionHostId, "hostid")
	resourceAttributes.PutStr(attr.ResourceDetectionHostName, "hostname")
	resourceAttributes.PutStr(attr.ResourceDetectionASG, "asg")

	resolver.Process(attributes, resourceAttributes)
	expectedInstanceId, ok := attributes.Get(attr.EC2InstanceId)
	assert.True(t, ok)
	assert.Equal(t, "hostid", expectedInstanceId.AsString())

	expectedHostName, ok := attributes.Get(attr.ResourceDetectionHostName)
	assert.True(t, ok)
	assert.Equal(t, "hostname", expectedHostName.AsString())

	expectedASG, ok := attributes.Get(attr.EC2AutoScalingGroupName)
	assert.True(t, ok)
	assert.Equal(t, "asg", expectedASG.AsString())
}
