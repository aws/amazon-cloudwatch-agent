// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"

	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/internal/attributes"
)

const AttributePlatformEC2 = "EC2"

// EC2HostedInAttributes is an allow-list that also renames attributes from the resource detection processor
var EC2HostedInAttributes = map[string]string{
	attr.ResourceDetectionHostId:   attr.EC2InstanceId,
	attr.ResourceDetectionHostName: attr.ResourceDetectionHostName,
	attr.ResourceDetectionASG:      attr.EC2AutoScalingGroupName,
}

type ec2HostedInAttributeResolver struct {
	name         string
	attributeMap map[string]string
}

func newEC2HostedInAttributeResolver(name string) *ec2HostedInAttributeResolver {
	if name == "" {
		name = AttributePlatformEC2
	}
	return &ec2HostedInAttributeResolver{
		name:         name,
		attributeMap: EC2HostedInAttributes,
	}
}
func (h *ec2HostedInAttributeResolver) Process(attributes, resourceAttributes pcommon.Map) error {
	for attrKey, mappingKey := range h.attributeMap {
		if val, ok := resourceAttributes.Get(attrKey); ok {
			attributes.PutStr(mappingKey, val.AsString())
		}
	}

	// If aws.hostedin.environment is populated, override HostedIn.EC2.Environment value
	// Otherwise, keep ASG name if it exists
	if val, ok := resourceAttributes.Get(attr.AWSHostedInEnvironment); ok {
		attributes.PutStr(attr.HostedInEC2Environment, val.AsString())
	} else if val, ok := resourceAttributes.Get(attr.ResourceDetectionASG); ok {
		attributes.PutStr(attr.HostedInEC2Environment, val.AsString())
	} else {
		attributes.PutStr(attr.HostedInEC2Environment, h.name)
	}

	return nil
}

func (h *ec2HostedInAttributeResolver) Stop(ctx context.Context) error {
	return nil
}
