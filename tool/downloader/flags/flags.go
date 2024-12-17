// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package flags

import "github.com/aws/amazon-cloudwatch-agent/tool/cmdwrapper"

const Command = "config-downloader"

var DownloaderFlags = map[string]cmdwrapper.Flag{
	"mode":            {DefaultValue: "ec2", Description: "Please provide the mode, i.e. ec2, onPremise, onPrem, auto"},
	"download-source": {DefaultValue: "", Description: "Download source. Example: \"ssm:my-parameter-store-name\" for an EC2 SSM Parameter Store Name holding your CloudWatch Agent configuration."},
	"output-dir":      {DefaultValue: "", Description: "Path of output json config directory."},
	"config":          {DefaultValue: "", Description: "Please provide the common-config file"},
	"multi-config":    {DefaultValue: "default", Description: "valid values: default, append, remove"},
}
