// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rules

import "go.opentelemetry.io/collector/pdata/pcommon"

func generateTestAttributes(service string, operation string, remoteService string, remoteOperation string,
	isTrace bool) pcommon.Map {
	attributes := pcommon.NewMap()
	if isTrace {
		attributes.PutStr("aws.local.service", service)
		attributes.PutStr("aws.local.operation", operation)
		attributes.PutStr("aws.remote.service", remoteService)
		attributes.PutStr("aws.remote.operation", remoteOperation)
	} else {
		attributes.PutStr("Service", service)
		attributes.PutStr("Operation", operation)
		attributes.PutStr("RemoteService", remoteService)
		attributes.PutStr("RemoteOperation", remoteOperation)
	}
	return attributes
}
