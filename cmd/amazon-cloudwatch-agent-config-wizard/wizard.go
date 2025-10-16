// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/tool/cmdwrapper"
	"github.com/aws/amazon-cloudwatch-agent/tool/wizard/flags"
)

func main() {
	fmt.Println("Starting config-wizard, this will map back to a call to amazon-cloudwatch-agent")

	fs, wizardFlags := cmdwrapper.CreateFlagSet(flags.Command, flags.WizardFlags)
	fs.Parse(os.Args[1:]) // Skip program name only

	err := cmdwrapper.ExecuteSubcommand(flags.Command, wizardFlags)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
