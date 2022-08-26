// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build integration
// +build integration

package test

import (
	"os"
	"os/user"
	"strconv"
	"log"
	"syscall"
)

func CheckFilePermissionAndOwner(filePath, level string) (filePermission string, fileUserOwner string, fileGroupOwner string){
	fileInfo, err := os.Stat(filePath)

	if (err != nil){
		log.Fatalf("Stat file %v failed due to %v", filePath, err)
	}

	uid, gid := getOwnership(fileInfo)
	fileUserOwner, fileGroupOwner, err = getNames(uid, gid)

	if (err != nil){
		log.Fatalf("Get user and group ownership failed because of %v", err)
	}

	filePermission = getFilePermissionAccordingToLevel(fileInfo, level)
	return
}

func getNames(uid, gid uint32) (string, string, error) {
	usr := strconv.FormatUint(uint64(uid), 10)
	group := strconv.FormatUint(uint64(gid), 10)
	if u, err := user.LookupId(usr); err == nil {
		usr = u.Username
	} else {
		return "", "", err
	}
	if g, err := user.LookupGroupId(group); err == nil {
		group = g.Name
	} else {
		return "", "", err
	}
	return usr, group, nil
}

func getOwnership(info os.FileInfo) (uid, gid uint32) {
	// https://golang.org/pkg/syscall/#Stat_t
	stat := info.Sys().(*syscall.Stat_t)
	return stat.Uid, stat.Gid
}

func getFilePermissionAccordingToLevel(info os.FileInfo, level string) string{
	permission :=  info.Mode()

	switch level {
	case "owner":
		return string(permission.String()[1:3])
	case "group":
		return string(permission.String()[4:7])
	default:
		return string(permission.String()[7:10])
	}
}