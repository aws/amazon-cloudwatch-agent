// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package paths

const (
	AgentDir             = "\\Amazon\\AmazonCloudWatchAgent\\"
	JsonDir              = "\\Configs"
	BinaryDir            = "bin"
	TranslatorBinaryName = "config-translator.exe"
	AgentBinaryName      = "amazon-cloudwatch-agent.exe"
	WizardBinaryName     = "amazon-cloudwatch-agent-config-wizard.exe"
	AgentStartName       = "amazon-cloudwatch-agent-ctl.ps1"
)
