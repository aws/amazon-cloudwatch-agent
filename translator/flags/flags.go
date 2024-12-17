// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package flags

import "github.com/aws/amazon-cloudwatch-agent/tool/cmdwrapper"

const TranslatorCommand = "config-translator"

var TranslatorFlags = map[string]cmdwrapper.Flag{
	"os":           {DefaultValue: "", Description: "Please provide the os preference, valid value: windows/linux."},
	"input":        {DefaultValue: "", Description: "Please provide the path of input agent json config file"},
	"input-dir":    {DefaultValue: "", Description: "Please provide the path of input agent json config directory."},
	"output":       {DefaultValue: "", Description: "Please provide the path of the output CWAgent config file"},
	"mode":         {DefaultValue: "ec2", Description: "Please provide the mode, i.e. ec2, onPremise, onPrem, auto"},
	"config":       {DefaultValue: "", Description: "Please provide the common-config file"},
	"multi-config": {DefaultValue: "remove", Description: "valid values: default, append, remove"},
}
