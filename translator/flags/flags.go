// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package flags

import "github.com/aws/amazon-cloudwatch-agent/tool/cmdwrapper"

const TranslatorCommand = "config-translator"

var TranslatorFlags = map[string]cmdwrapper.Flag{
	"os":           {"os", "", "Please provide the os preference, valid value: windows/linux."},
	"input":        {"input", "", "Please provide the path of input agent json config file"},
	"input-dir":    {"input-dir", "", "Please provide the path of input agent json config directory."},
	"output":       {"output", "", "Please provide the path of the output CWAgent config file"},
	"mode":         {"mode", "ec2", "Please provide the mode, i.e. ec2, onPremise, onPrem, auto"},
	"config":       {"config", "", "Please provide the common-config file"},
	"multi-config": {"multi-config", "remove", "valid values: default, append, remove"},
}
