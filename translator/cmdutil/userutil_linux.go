// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// +build linux

package cmdutil

import (
	"fmt"
	"log"
	"os"
	gouser "os/user"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/runc/libcontainer/user"
)

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

	g, err := gouser.LookupGroupId(strconv.Itoa(execUser.Gid))
	if err != nil {
		return runAsUser, fmt.Errorf("error lookup group by id: %w", err)
	}
	if err := changeFileOwner(runAsUser, g.Name); err != nil {
		return runAsUser, fmt.Errorf("error change ownership of dirs: %w", err)
	}

	if runAsUser == "root" {
		return "root", nil
	}

	if err := switchUser(execUser); err != nil {
		log.Printf("E! failed switching to %q: %v", runAsUser, err)
		return runAsUser, err
	}

	return runAsUser, nil
}
