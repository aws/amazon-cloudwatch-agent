// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rules

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/attributes"
)

func generateTestAttributes(service string, operation string, remoteService string, remoteOperation string,
	isTrace bool) pcommon.Map {
	return generateAttributesWithEnv(service, operation, "", remoteService, remoteOperation, "", isTrace)
}

func generateAttributesWithEnv(service string, operation string, environment string,
	remoteService string, remoteOperation string, remoteEnvironment string,
	isTrace bool) pcommon.Map {
	attributes := pcommon.NewMap()
	if isTrace {
		attributes.PutStr(attr.AWSLocalService, service)
		attributes.PutStr(attr.AWSLocalOperation, operation)
		if environment != "" {
			attributes.PutStr(attr.AWSLocalEnvironment, environment)
		}
		attributes.PutStr(attr.AWSRemoteService, remoteService)
		attributes.PutStr(attr.AWSRemoteOperation, remoteOperation)
		if remoteEnvironment != "" {
			attributes.PutStr(attr.AWSRemoteEnvironment, remoteEnvironment)
		}
	} else {
		attributes.PutStr(common.CWMetricAttributeLocalService, service)
		attributes.PutStr(common.CWMetricAttributeLocalOperation, operation)
		if environment != "" {
			attributes.PutStr(common.CWMetricAttributeEnvironment, environment)
		}
		attributes.PutStr(common.CWMetricAttributeRemoteService, remoteService)
		attributes.PutStr(common.CWMetricAttributeRemoteOperation, remoteOperation)
		if remoteEnvironment != "" {
			attributes.PutStr(common.CWMetricAttributeRemoteEnvironment, remoteEnvironment)
		}
	}
	return attributes
}
