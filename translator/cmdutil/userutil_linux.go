// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// +build linux

package cmdutil

import (
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"os"
	"os/user"
	"strconv"
)

type ExecUser struct {
	Uid  int
	Gid  int
	Home string
	Gids []int
}

func getAllGids(u *user.User) ([]int, error) {
	groups, err := u.GroupIds()
	if err != nil {
		return nil, err
	}
	gids := make([]int, len(groups))
	for _, group := range groups {
		gid, err := strconv.Atoi(group)
		if err != nil {
			log.Printf("E! Failed to convert group to int: %v", err)
			return nil, err
		}
		gids = append(gids, gid)
	}
	return gids, nil
}

func toExecUser(u *user.User) (*ExecUser, error) {
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		log.Printf("E! Failed to convert uid to int: %v", err)
		return nil, err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		log.Printf("E! Failed to convert gid to int: %v", err)
		return nil, err
	}
	gids, err := getAllGids(u)
	if err != nil {
		log.Printf("E! Failed to get group IDs: %v", err)
		return nil, err
	}
	return &ExecUser{Uid: uid, Gid: gid, Home: u.HomeDir, Gids: gids}, nil
}

func switchUser(execUser *ExecUser) error {
	if err := unix.Setgroups(execUser.Gids); err != nil {
		log.Printf("E! Failed to set groups: %v", err)
		return err
	}

	if err := setGid(execUser.Gid); err != nil {
		log.Printf("E! Failed to set gid: %v", err)
		return err
	}

	if err := setUid(execUser.Uid); err != nil {
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

func getRunAsExecUser(runasuser string) (*ExecUser, error) {
	newUser, err := user.Lookup(runasuser)
	if err != nil {
		log.Printf("E! Failed to get newUser: %v", err)
		return nil, err
	}
	return toExecUser(newUser)
}

func ChangeUser(mergedJsonConfigMap map[string]interface{}) (string, error) {
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

	if err := changeFileOwner(execUser.Uid, execUser.Gid); err != nil {
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
