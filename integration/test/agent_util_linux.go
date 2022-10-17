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

func DeleteFile(filePathAbsolute string) error {
	log.Printf("Delete file %s", filePathAbsolute)
	out, err := exec.Command("bash", "-c", "sudo rm "+filePathAbsolute).Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
		return err
	}

	log.Printf("Removed file: %s", filePathAbsolute)
	return nil
}

func StartAgent(configOutputPath string, fatalOnFailure bool) error {
	out, err := exec.
		Command("bash", "-c", "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a fetch-config -m ec2 -s -c file:"+configOutputPath).
		Output()

	if err != nil && fatalOnFailure {
		log.Fatal(fmt.Sprint(err) + string(out))
	} else if err != nil {
		log.Printf(fmt.Sprint(err) + string(out))
	} else {
		log.Printf("Agent has started")
	}

	return err
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

func RunShellScript(path string, args ...string) error {
	out, err := exec.Command("bash", "-c", "chmod +x "+path).Output()

	if err != nil {
		log.Printf("Error occurred when attempting to chmod %s: %s | %s", path, err.Error(), string(out))
		return err
	}

	bashArgs := []string{"-c", "sudo ./" + path}
	bashArgs = append(bashArgs, args...)

	//out, err = exec.Command("bash", "-c", "sudo ./"+path, args).Output()
	out, err = exec.Command("bash", bashArgs...).Output()

	if err != nil {
		log.Printf("Error occurred when executing %s: %s | %s", path, err.Error(), string(out))
		return err
	}

	return nil
}

func RunCommand(cmd string) {
	out, err := exec.Command("bash", "-c", cmd).Output()

	if err != nil {
		log.Fatalf("Error occurred when executing %s: %s | %s", cmd, err.Error(), string(out))
	}
}

func ReplaceLocalStackHostName(pathIn string) {
	out, err := exec.Command("bash", "-c", "sed -i 's/localhost.localstack.cloud/'\"$LOCAL_STACK_HOST_NAME\"'/g' "+pathIn).Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}
}
