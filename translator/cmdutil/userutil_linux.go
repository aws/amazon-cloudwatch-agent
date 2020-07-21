// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// +build linux

package cmdutil

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/runc/libcontainer/user"
	"golang.org/x/sys/unix"
	"log"
	"os"
	"os/exec"
	gouser "os/user"
	"strconv"
	"syscall"
)

func DetectRunAsUser(mergedJsonConfigMap map[string]interface{}) (runAsUser string, err error) {
	fmt.Printf("I! Detecting runasuser...\n")
	if agentSection, ok := mergedJsonConfigMap["agent"]; ok {
		agent := agentSection.(map[string]interface{})
		if user, ok := agent["run_as_user"]; ok {
			if runasuser, ok := user.(string); ok {
				return runasuser, nil
			}
			fmt.Printf("E! runasuser is not string %v \n", user)
			panic("E! runasuser is not string \n")
		}

		// agent section exists, but "runasuser" does not exist, then use "root"
		return "root", nil
	}

	// no agent section, it means no runasuser, use "root"
	return "root", nil
}

func switchUser(execUser *user.ExecUser) error {
	if err := unix.Setgroups(execUser.Sgids); err != nil {
		log.Printf("E! Failed to set groups: %v", err)
		return err
	}

	if err := system.Setgid(execUser.Gid); err != nil {
		log.Printf("E! Failed to set gid: %v", err)
		return err
	}

	if err := system.Setuid(execUser.Uid); err != nil {
		log.Printf("E! Failed to set uid: %v", err)
		return err
	}

	if err := os.Setenv("HOME", execUser.Home); err != nil {
		log.Printf("E! Failed to set HOME: %v", err)
		return err
	}
	log.Printf("I! Set HOME: %v", execUser.Home)

	return nil
}

func getRunAsExecUser(runasuser string) (*user.ExecUser, error) {
	currExecUser := user.ExecUser{
		Uid:  syscall.Getuid(),
		Gid:  syscall.Getgid(),
		Home: "/root",
	}
	newUser, err := user.GetExecUserPath(runasuser, &currExecUser, "/etc/passwd", "/etc/group")
	if err != nil {
		log.Printf("E! Failed to get newUser: %v", err)
		return nil, err
	}
	return newUser, nil
}

func ChangeUser(mergedJsonConfigMap map[string]interface{}) (user string, err error) {
	runAsUser, _ := DetectRunAsUser(mergedJsonConfigMap)
	log.Printf("I! Detected runAsUser: %v", runAsUser)
	if runAsUser == "" {
		runAsUser = "root"
	}

	execUser, err := getRunAsExecUser(runAsUser)
	if err != nil {
		log.Printf("E! Failed to getRunAsExecUser: %v", err)
		return runAsUser, err
	}

	changeFileOwner(runAsUser, execUser.Gid)

	if runAsUser == "root" {
		return "root", nil
	}

	if err := switchUser(execUser); err != nil {
		log.Printf("E! failed switching to %q: %v", runAsUser, err)
		return runAsUser, err
	}

	return runAsUser, nil
}

func changeFileOwner(runAsUser string, groupId int) {
	group, err := gouser.LookupGroupId(strconv.Itoa(groupId))
	owner := runAsUser
	if err == nil {
		owner = owner + ":" + group.Name
	} else {
		log.Printf("I! Failed to get the group name: %v, it will just change the user, but not group.", err)
	}
	log.Printf("I! Change ownership to %v", owner)

	chowncmd := exec.Command("chown", "-R", "-L", owner, "/opt/aws/amazon-cloudwatch-agent/logs")
	stdoutStderr, err := chowncmd.CombinedOutput()
	if err != nil {
		log.Printf("E! Change ownership of /opt/aws/amazon-cloudwatch-agent/logs: %s %v", stdoutStderr, err)
	}

	chowncmd = exec.Command("chown", "-R", "-L", owner, "/opt/aws/amazon-cloudwatch-agent/etc")
	stdoutStderr, err = chowncmd.CombinedOutput()
	if err != nil {
		log.Printf("E! Change ownership of /opt/aws/amazon-cloudwatch-agent/etc: %s %v", stdoutStderr, err)
	}

	chowncmd = exec.Command("chown", "-R", "-L", owner, "/opt/aws/amazon-cloudwatch-agent/var")
	stdoutStderr, err = chowncmd.CombinedOutput()
	if err != nil {
		log.Printf("E! Change ownership of /opt/aws/amazon-cloudwatch-agent/var: %s %v", stdoutStderr, err)
	}
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
