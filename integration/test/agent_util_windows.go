// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows && integration
// +build windows,integration

package test

import (
	"os/exec"
	"log"
	"fmt"
	"path/filepath"
)

func CopyFile(pathIn string, pathOut string) {
	log.Printf("Copy File %s to %s", pathIn, pathOut)
	pathInAbs, err := filepath.Abs(pathIn)

	if err != nil {
		log.Fatalf(err)
	}

	log.Printf("File %s abs path %s", pathIn, pathInAbs)
	out, err := exec.Command("cp "+pathInAbs+" "+pathOut).Output()

	if err != nil {
		log.Fatalf("Copy file failed: %v; the output is: %s",err, string(out))
	}

	log.Printf("File : %s copied to : %s", pathIn, pathOut)
	
}

func StartAgent(configOutputPath string, fatalOnFailure bool) error {
	ps, err := exec.LookPath("powershell.exe")

	if err != nil {
		return err
	}

	bashArgs := append([]string{"-NoProfile", "-NonInteractive", "-NoExit", "& \"C:\\Program Files\\Amazon\\AmazonCloudWatchAgent\\amazon-cloudwatch-agent-ctl.ps1\" -a fetch-config -m ec2 -s -c file:"+configOutputPath})
	out, err := exec.Command(ps, bashArgs...).Output()

	if err != nil && fatalOnFailure {
		return fmt.Errorf("Start agent failed: %v; the output is: %s",err, string(out))
	} else if err != nil {
		log.Printf(fmt.Sprint(err) + string(out))
	} else {
		log.Printf("Agent has started")
	}

	return err
}

func StopAgent() error{
	ps, err := exec.LookPath("powershell.exe")

	if err != nil {
		return err
	}

	bashArgs := append([]string{"-NoProfile", "-NonInteractive", "-NoExit", "& \"C:\\Program Files\\Amazon\\AmazonCloudWatchAgent\\amazon-cloudwatch-agent-ctl.ps1\" -a stop"})
	out, err := exec.Command(ps, bashArgs...).Output()

	if err != nil {
		return err
	}

	log.Printf("Agent is stopped")
	return nil
}

func RunShellScript(path string, args ...string) error{
	ps, err := exec.LookPath("powershell.exe")

	if err != nil {
		return err
	}

	bashArgs := append([]string{"-NoProfile", "-NonInteractive", "-NoExit", path}, args...)
	out, err := exec.Command(ps, bashArgs...).Output()

	if err != nil {
		log.Printf("Error occurred when executing %s: %s | %s", path, err.Error(), string(out))
		return err
	}

	return nil
}