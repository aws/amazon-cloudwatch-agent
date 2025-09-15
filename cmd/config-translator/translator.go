// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"log"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/tool/cmdwrapper"
	"github.com/aws/amazon-cloudwatch-agent/translator/flags"
)

func main() {
	log.Printf("Starting config-translator, this will map back to a call to amazon-cloudwatch-agent")

	fs, translatorFlags := cmdwrapper.CreateFlagSet(flags.TranslatorCommand, flags.TranslatorFlags)
	fs.Parse(os.Args[1:]) // Skip program name only

	err := cmdwrapper.ExecuteSubcommand(flags.TranslatorCommand, translatorFlags)
	if err != nil {
		os.Exit(1)
	}
}
