// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

// This is used for windows_perf_counters, which requires instances field,
// but for some Object who doesn't support instances, we still need to specify
// Instances = ["------"]

// Ideally we don't need to check this, if customer doesn't specify resource, we assume there is no instance, and use "------"
// This is for backwards compatible purpose, if customer using these windows Object names, and specify instance using "*" or something else, we don't want eliminate their metric silently
// TODO: fail the translation if we find customer provide resources fields, after that we can remove this check. https://github.com/aws/amazon-cloudwatch-agent/issues/232

var Instances_disabled_plugins = []string{
	"System",
	"Memory",
	"TCPv4",
	"TCPv6",
}
