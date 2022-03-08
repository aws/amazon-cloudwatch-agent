// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build darwin
// +build darwin

package cmdutil

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

// userutil_darwin.go contains macOS specific logic and hacks.
// The root cause lies in os/user in standard library, See https://github.com/golang/go/issues/24383#issuecomment-372908869.
// In short os/user only has the cgo version working on macOS.
// macOS does not save all the users and groups in /etc/passwd and /etc/groups like other Unix systems.
// However, os/user does not break many user because the logic for looking for current user is another route.
// We adopted the approach from https://github.com/tweekmonster/luser because we only need the macOS part.
// NOTE(pingleig): We should remove those hacks if we are able to persuade the upstream to accept the fix.

const (
	// dsbin is a cli for querying macOS directory service.
	dsbin = "/usr/bin/dscacheutil"
)

// dsLookup shells out to dscacheutil to get uid, gid from username.
func dsLookup(username string) (*user.User, error) {
	// dscacheutil -q user -a name cwagent
	// name: cwagent
	// password: ********
	// uid: 1881561228
	// gid: 2896053708
	// dir: /Users/cwagent
	// shell: /bin/zsh
	// gecos: CloudWatch, Agent
	//
	m, err := runDS("-q", "user", "-a", "name", username)
	if err != nil {
		return nil, err
	}
	u := &user.User{
		Uid:      m["uid"],
		Gid:      m["gid"],
		Username: m["name"],
		Name:     m["gecos"],
		HomeDir:  m["dir"],
	}
	if u.Username == "" || u.Username != username {
		return nil, user.UnknownUserError(username)
	}
	return u, nil
}

// runDS shells out query to dscacheutil and parse it to key value pair.
func runDS(args ...string) (map[string]string, error) {
	b, err := exec.Command(dsbin, args...).CombinedOutput()
	if err != nil {
		cmd := strings.Join(append([]string{dsbin}, args...), " ")
		return nil, fmt.Errorf("error query directory service using %s: %w output %s", cmd, err, b)
	}
	return parseDSOutput(string(b))
}

// parseDSOutput splits dscacheutil output into key value pair.
// It returns error if no pair is found.
func parseDSOutput(s string) (map[string]string, error) {
	const sep = ": "
	lines := strings.Split(s, "\n")
	m := make(map[string]string)
	for _, line := range lines {
		keyEnd := strings.Index(line, sep)
		if keyEnd <= 0 { // the name must be longer then 1, i.e. `: value` does not exist
			continue
		}
		m[line[:keyEnd]] = line[keyEnd+len(sep):]
	}
	if len(m) == 0 {
		return m, fmt.Errorf("error parse %s output %s", dsbin, s)
	}
	return m, nil
}

func switchUser(execUser *user.User) error {
	gid, _ := strconv.Atoi(execUser.Gid)
	if err := syscall.Setgid(gid); err != nil {
		log.Printf("E! Failed to set gid: %v", err)
		return err
	}

	uid, err := strconv.Atoi(execUser.Uid)
	if err != nil {
		return fmt.Errorf("id is not a valid integer %w", err)
	}

	if err := syscall.Setuid(uid); err != nil {
		log.Printf("E! Failed to set uid: %v", err)
		return err
	}

	if err := os.Setenv("HOME", execUser.HomeDir); err != nil {
		log.Printf("E! Failed to set HOME: %v", err)
		return err
	}
	log.Printf("I! Set HOME: %v", execUser.HomeDir)

	return nil
}

func ChangeUser(mergedJsonConfigMap map[string]interface{}) (string, error) {
	runAsUser, _ := DetectRunAsUser(mergedJsonConfigMap)
	log.Printf("I! Detected runAsUser: %v", runAsUser)
	if runAsUser == "" {
		return "root", nil
	}

	execUser, err := dsLookup(runAsUser)
	if err != nil {
		log.Printf("E! Failed to getRunAsExecUser: %v", err)
		return runAsUser, err
	}

	uid, err := strconv.Atoi(execUser.Uid)
	if err != nil {
		return runAsUser, fmt.Errorf("UID %s cannot be converted to integer uid: %w", execUser.Uid, err)
	}

	gid, err := strconv.Atoi(execUser.Gid)
	if err != nil {
		log.Printf("W! GID %s cannot be converted to integer gid, not changing gid of files: %v", execUser.Gid, err)
		gid = -1 // -1 means not changing the GUID, see: https://golang.org/pkg/os/#Chown
	}

	if err := changeFileOwner(uid, gid); err != nil {
		return runAsUser, fmt.Errorf("error change ownership of dirs: %w", err)
	}

	fmt.Printf("I! user found is %+v\n", execUser.Username)
	if err := switchUser(execUser); err != nil {
		log.Printf("E! failed switching to %q: %v", runAsUser, err)
		return runAsUser, err
	}

	return runAsUser, nil
}
