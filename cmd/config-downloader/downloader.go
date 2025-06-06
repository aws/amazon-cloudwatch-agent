// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package main

import (
	"flag"
	"log"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/tool/cmdwrapper"
	"github.com/aws/amazon-cloudwatch-agent/tool/downloader/flags"
)

func main() {
	log.Printf("Starting config-downloader, this will map back to a call to amazon-cloudwatch-agent")

	translatorFlags := cmdwrapper.AddFlags("", flags.DownloaderFlags)
	flag.Parse()

	err := cmdwrapper.ExecuteAgentCommand(flags.Command, translatorFlags)
	if err != nil {
		os.Exit(1)
	}
}
