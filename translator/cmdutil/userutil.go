// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || darwin
// +build linux darwin

package cmdutil

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

var (
	agentLogDir = "/opt/aws/amazon-cloudwatch-agent/logs"
	agentVarDir = "/opt/aws/amazon-cloudwatch-agent/var"
	agentEtcDir = "/opt/aws/amazon-cloudwatch-agent/etc"
)

type ChownFunc func(name string, uid, gid int) error

var chown ChownFunc = os.Chown

// DetectRunAsUser get the user name from toml config. It runs on all platforms except windows.
func DetectRunAsUser(mergedJsonConfigMap map[string]interface{}) (runAsUser string, err error) {
	fmt.Printf("I! Detecting run_as_user...\n")
	if agentSection, ok := mergedJsonConfigMap["agent"]; ok {
		agent := agentSection.(map[string]interface{})
		if user, ok := agent["run_as_user"]; ok {
			if runasuser, ok := user.(string); ok {
				return runasuser, nil
			}

			log.Panicf("E! run_as_user is not string %v", user)
		}

		// agent section exists, but "runasuser" does not exist, then use "root"
		return "root", nil
	}

	// no agent section, it means no runasuser, use "root"
	return "root", nil
}

// changeFileOwner changes both user and group of a directory.
func changeFileOwner(uid, gid int) error {
	dirs := []string{agentLogDir, agentEtcDir, agentVarDir}
	log.Printf("I! Changing ownership of %v to %v:%v", dirs, uid, gid)
	for _, d := range dirs {
		if err := chownRecursive(uid, gid, d); err != nil {
			return err
		}
	}
	return nil
}

// chownRecursive would recursively change the ownership of the directory
// similar to `chown -R <dir>`, except it will igore any files that are:
//   - Executable
//   - With SUID or SGID bit set
//   - Allow anyone to write to
//   - Symbolic links
//
// This would prevent any accidental ownership change to files that are executable
// or with special purpose to be changed to be owned by root when run_as_user option
// is removed from the configuration
func chownRecursive(uid, gid int, dir string) error {

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		fmode := info.Mode()
		if fmode.IsRegular() {
			// Do not change ownership of files with SUID or SGID
			if fmode&os.ModeSetuid != 0 || fmode&os.ModeSetgid != 0 {
				return nil
			}

			// Do not change ownership of executable files
			// Perm() returns the lower 7 bit of permission of file, which represes rwxrwxrws
			// 0111 maps to --x--x--x, so it would check any user have the execution right
			if fmode.Perm()&0111 != 0 {
				return nil
			}

			// No need to change ownership of files that allow anyone to write to
			if fmode.Perm()&0002 != 0 {
				return nil
			}
		}

		if fmode&os.ModeSymlink != 0 {
			return nil
		}

		if err := chown(path, uid, gid); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error change owner of dir %s to %v:%v due to error: %w", dir, uid, gid, err)
	}
	return nil
}

func VerifyCredentials(ctx *context.Context, runAsUser string) {
	credentials := ctx.Credentials()
	if (config.ModeOnPrem == ctx.Mode()) || (config.ModeOnPremise == ctx.Mode()) {
		if runAsUser != "root" {
			if _, ok := credentials["shared_credential_file"]; !ok {
				log.Panic("E! Credentials path is not set while runasuser is not root")
			}
		}
	}
}
