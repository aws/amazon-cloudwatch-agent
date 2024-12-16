// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package flags

import "github.com/aws/amazon-cloudwatch-agent/tool/cmdwrapper"

const Command = "config-downloader"

var DownloaderFlags = map[string]cmdwrapper.Flag{
	"mode":            {"mode", "ec2", "Please provide the mode, i.e. ec2, onPremise, onPrem, auto"},
	"download-source": {"download-source", "", "Download source. Example: \"ssm:my-parameter-store-name\" for an EC2 SSM Parameter Store Name holding your CloudWatch Agent configuration."},
	"output-dir":      {"output-dir", "", "Path of output json config directory."},
	"config":          {"config", "", "Please provide the common-config file"},
	"multi-config":    {"multi-config", "default", "valid values: default, append, remove"},
}
