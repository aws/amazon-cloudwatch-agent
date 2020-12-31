// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// +build linux darwin

package cmdutil

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

const (
	agentLogDir = "/opt/aws/amazon-cloudwatch-agent/logs"
	agentVarDir = "/opt/aws/amazon-cloudwatch-agent/var"
	agentEtcDir = "/opt/aws/amazon-cloudwatch-agent/etc"
)

// DetectRunAsUser get the user name from toml config. It runs on all platforms except windows.
func DetectRunAsUser(mergedJsonConfigMap map[string]interface{}) (runAsUser string, err error) {
	fmt.Printf("I! Detecting run_as_user...\n")
	if agentSection, ok := mergedJsonConfigMap["agent"]; ok {
		agent := agentSection.(map[string]interface{})
		if user, ok := agent["run_as_user"]; ok {
			if runasuser, ok := user.(string); ok {
				return runasuser, nil
			}
			fmt.Printf("E! run_as_user is not string %v \n", user)
			panic("E! run_as_user is not string \n")
		}

		// agent section exists, but "runasuser" does not exist, then use "root"
		return "root", nil
	}

	// no agent section, it means no runasuser, use "root"
	return "root", nil
}

// changeFileOwner changes both user and group of a directory.
func changeFileOwner(runAsUser string, groupName string) error {
	owner := runAsUser
	if groupName != "" {
		owner = owner + ":" + groupName
	} else {
		log.Print("W! Group name is empty, change user without group.")
	}
	dirs := []string{agentLogDir, agentEtcDir, agentVarDir}
	log.Printf("I! Changing ownership of %v to %s", dirs, owner)
	for _, d := range dirs {
		if err := chownRecursive(owner, d); err != nil {
			return err
		}
	}
	return nil
}

// chownRecursive shells out to chown -R -L
func chownRecursive(owner, dir string) error {
	cmd := exec.Command("chown", "-R", "-L", owner, dir)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error change owner of dir %s to %s: %w %s", dir, owner, err, b)
	}
	return nil
}

func VerifyCredentials(ctx *context.Context, runAsUser string) {
	credentials := ctx.Credentials()
	if config.ModeOnPrem == ctx.Mode() {
		if runAsUser != "root" {
			if _, ok := credentials["shared_credential_file"]; !ok {
				panic("E! Credentials path is not set while runasuser is not root \n")
			}
		}
	}
}
