// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows && integration
// +build windows,integration

package test

import (
	"os/exec"
)

func RunPowerShellScript(path string, args ...string) error{
	ps, err := exec.LookPath("powershell.exe")

	if err != nil {
		return err
	}

	bashArgs := append([]string{"-NoProfile", "-NonInteractive", "-NoExit", path}, args...)
	out, err := exec.Command(ps, bashArgs...).Output()

	if err != nil {
		log.Fatalf("Error occurred when executing %s: %s | %s", path, err.Error(), string(out))
		return err
	}

	return nil
}

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

func RunCommand(cmd string) {
	out, err := exec.Command("bash", "-c", cmd).Output()

	if err != nil {
		log.Fatalf("Error occurred when executing %s: %s | %s", cmd, err.Error(), string(out))
	}
}