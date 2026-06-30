// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package opentelemetry

var logsCleanupStatements = []string{
	`delete_key(resource.attributes, "aws.log.group.name")`,
	`delete_key(resource.attributes, "aws.log.stream.name")`,
	`delete_key(resource.attributes, "aws.log.source")`,
}
