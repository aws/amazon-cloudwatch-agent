// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"log"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/tool/cmdwrapper"
	"github.com/aws/amazon-cloudwatch-agent/translator/flags"
)

func main() {
	log.Printf("Starting config-translator, this will map back to a call to amazon-cloudwatch-agent")

	translatorFlags := cmdwrapper.AddFlags("", flags.TranslatorFlags)
	flag.Parse()

	err := cmdwrapper.ExecuteAgentCommand(flags.TranslatorCommand, translatorFlags)
	if err != nil {
		os.Exit(1)
	}
}
