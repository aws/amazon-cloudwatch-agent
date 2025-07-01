// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"log"

	"github.com/aws/amazon-cloudwatch-agent/tool/cmdwrapper"
	"github.com/aws/amazon-cloudwatch-agent/tool/wizard/flags"
)

func main() {
	log.Printf("Starting config-wizard, this will map back to a call to amazon-cloudwatch-agent")

	translatorFlags := cmdwrapper.AddFlags("", flags.WizardFlags)
	flag.Parse()

	_ = cmdwrapper.ExecuteAgentCommand(flags.Command, translatorFlags)
}
