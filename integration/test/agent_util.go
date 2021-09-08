// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// +build linux
// +build integration

package test

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"
)

func CopyFile(pathIn string, pathOut string) {
	log.Printf("Copy File %s to %s", pathIn, pathOut)
	pathInAbs, err := filepath.Abs(pathIn)
	log.Printf("File %s abs path %s", pathIn, pathInAbs)
	if err != nil {
		log.Fatal(err)
	}
	command := exec.Command("bash", "-c", "sudo cp " + pathInAbs + " " + pathOut)
	err = command.Run()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("File : %s copied to : %s", pathIn, pathOut);
}

func StartAgent(configOutputPath string) {
	command := exec.Command("bash", "-c", "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a " +
		"fetch-config -m ec2 -s -c file:" +
		configOutputPath)

	err := command.Run()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Agent has started")
}

func StopAgent() {
	command := exec.Command("bash", "-c", "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a stop")
	err := command.Run()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Agent is stopped");
}

func ReadAgentOutput(d time.Duration) string {
	output, err := exec.Command("bash", "-c",
		fmt.Sprintf("journalctl -u amazon-cloudwatch-agent.service --since \"%s ago\" --no-pager", d.String())).Output()

	if err != nil {
		log.Fatal(err)
	}

	return string(output)
}
