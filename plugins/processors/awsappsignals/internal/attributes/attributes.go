// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package attributes

const (
	// aws attributes
	AWSLocalService             = "aws.local.service"
	AWSLocalOperation           = "aws.local.operation"
	AWSRemoteService            = "aws.remote.service"
	AWSRemoteOperation          = "aws.remote.operation"
	AWSRemoteTarget             = "aws.remote.target"
	AWSRemoteResourceIdentifier = "aws.remote.resource.identifier"
	AWSRemoteResourceType       = "aws.remote.resource.type"
	AWSHostedInEnvironment      = "aws.hostedin.environment"

	// resource detection processor attributes
	ResourceDetectionHostId   = "host.id"
	ResourceDetectionHostName = "host.name"
	ResourceDetectionASG      = "ec2.tag.aws:autoscaling:groupName"

	AWSLocalEnvironment  = "aws.local.environment"
	AWSRemoteEnvironment = "aws.remote.environment"
)
