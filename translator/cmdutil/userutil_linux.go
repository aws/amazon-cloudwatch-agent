// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux
// +build linux

package cmdutil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

type ExecUser struct {
	Uid  int
	Gid  int
	Home string
	Gids []int
}

func containsUser(users []string, match string) bool {
	for _, user := range users {
		if user == match {
			return true
		}
	}
	return false
}

func getGroupIds(user, filePath string) ([]int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("E! Failed to open group file: %v", err)
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	groupIds := []int{}
	for {
		var wholeLine []byte
		for {
			line, isPrefix, err := reader.ReadLine()

			if err != nil {
				// EOF reached
				if err == io.EOF {
					return groupIds, nil
				}
				return nil, err
			}

			// Whole line was able to fit in single buffer
			if !isPrefix && len(wholeLine) == 0 {
				wholeLine = line
				break
			}
			wholeLine = append(wholeLine, line...)
			// Last fragment of line read
			if !isPrefix {
				break
			}
		}

		wholeLine = bytes.TrimSpace(wholeLine)
		// Not empty, not a comment, and has enough parts
		if len(wholeLine) == 0 || wholeLine[0] == '#' || bytes.Count(wholeLine, []byte{':'}) < 3 {
			continue
		}
		parts := strings.SplitN(string(wholeLine), ":", 4)
		users := strings.Split(parts[3], ",")
		if len(users) == 0 || !containsUser(users, user) {
			continue
		}
		groupId, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, err
		}
		groupIds = append(groupIds, groupId)
	}
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
	gids, err := getGroupIds(u.Username, "/etc/group")
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
