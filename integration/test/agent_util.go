// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

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

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("File %s abs path %s", pathIn, pathInAbs)
	out, err := exec.Command("bash", "-c", "sudo cp "+pathInAbs+" "+pathOut).Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}

	log.Printf("File : %s copied to : %s", pathIn, pathOut)
}

func StartAgent(configOutputPath string) {
	out, err := exec.
		Command("bash", "-c", "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a fetch-config -m ec2 -s -c file:"+configOutputPath).
		Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}

	log.Printf("Agent has started")
}

func StopAgent() {
	out, err := exec.
		Command("bash", "-c", "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a stop").
		Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}

	log.Printf("Agent is stopped")
}

func ReadAgentOutput(d time.Duration) string {
	out, err := exec.Command("bash", "-c",
		fmt.Sprintf("journalctl -u amazon-cloudwatch-agent.service --since \"%s ago\" --no-pager", d.String())).
		Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}

	return string(out)
}

func RunShellScript(path string) {
	out, err := exec.Command("bash", "-c", "chmod +x "+path).Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}

	out, err = exec.Command("bash", "-c", "sudo ./"+path).Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}
}

func ReplaceLocalStackHostName(pathIn string) {
	out, err := exec.Command("bash", "-c", "sed -i 's/localhost.localstack.cloud/'\"$LOCAL_STACK_HOST_NAME\"'/g' " + pathIn).Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}
}
