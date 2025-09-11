// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package main

import (
	"log"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/tool/cmdwrapper"
	"github.com/aws/amazon-cloudwatch-agent/tool/downloader/flags"
)

func main() {
	log.Printf("Starting config-downloader, this will map back to a call to amazon-cloudwatch-agent")

	fs, downloaderFlags := cmdwrapper.CreateFlagSet(flags.Command, flags.DownloaderFlags)
	fs.Parse(os.Args[1:]) // Skip program name only

	_ = cmdwrapper.ExecuteSubcommand(flags.Command, downloaderFlags)
}
