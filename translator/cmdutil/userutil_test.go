// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package cmdutil

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type MockChowner struct {
	chowns []chownEvent
}

type chownEvent struct {
	path     string
	uid, gid int
}

func (c *MockChowner) Chown(path string, uid, gid int) error {
	c.chowns = append(c.chowns, chownEvent{path, uid, gid})
	return nil
}

func TestChangeFileOwner(t *testing.T) {
	base, err := os.MkdirTemp("", "testChown")
	if err != nil {
		t.Fatalf("failed to crate temp test folder: %v", err)
	}
	defer os.RemoveAll(base) // Post test clean up

	// Override the agent dirs
	agentLogDir = filepath.Join(base, "logs")
	agentVarDir = filepath.Join(base, "var")
	agentEtcDir = filepath.Join(base, "etc")

	linkedTo := filepath.Join(base, "file-to-be-linked-to.txt")
	createFile(t, linkedTo, 0644)

	// etc
	createFile(t, filepath.Join(agentEtcDir, "amazon-cloudwatch-agent.json"), 0644)
	createFile(t, filepath.Join(agentEtcDir, "amazon-cloudwatch-agent.toml"), 0644)
	createFile(t, filepath.Join(agentEtcDir, "common-config.toml"), 0644)
	createFile(t, filepath.Join(agentEtcDir, "amazon-cloudwatch-agent.d/default.json"), 0644)

	// var
	createFile(t, filepath.Join(agentVarDir, "some-file-in-var"), 0644)

	// Log files
	createFile(t, filepath.Join(agentLogDir, "amazon-cloudwatch-agent.log"), 0644)
	createFile(t, filepath.Join(agentLogDir, "state/statefile"), 0644)

	// Files that should be excluded
	createFile(t, filepath.Join(agentLogDir, "anyone-can-write.log"), 0666)
	createFile(t, filepath.Join(agentLogDir, "suid-file.log"), 0644|os.ModeSetuid)
	/*
		There is an issue in ibuild where g is not being set by chmod
		Add this line back to the test when there is a solution
		This commit does not have to be added to the open source cw agent test
		createFile(t, filepath.Join(agentLogDir, "sgid-file.log"), 0644|os.ModeSetgid)
	*/
	createFile(t, filepath.Join(agentLogDir, "suid-and-sgid-file.log"), 0644|os.ModeSetuid|os.ModeSetgid)
	createFile(t, filepath.Join(agentLogDir, "owner-executable.log"), 0744)
	createFile(t, filepath.Join(agentLogDir, "group-executable.log"), 0654)
	createFile(t, filepath.Join(agentLogDir, "other-executable.log"), 0645)
	createFile(t, filepath.Join(agentLogDir, "all-executable.log"), 0755)
	createSymlink(t, linkedTo, filepath.Join(agentLogDir, "link-to-other-file"))

	// Call changeFileOwner
	var mc MockChowner
	chown = mc.Chown
	const someUid, someGid = 1111, 9999
	changeFileOwner(someUid, someGid)

	expected := []chownEvent{
		{filepath.Join(base, "logs"), someUid, someGid},
		{filepath.Join(base, "logs/amazon-cloudwatch-agent.log"), someUid, someGid},
		{filepath.Join(base, "logs/state"), someUid, someGid},
		{filepath.Join(base, "logs/state/statefile"), someUid, someGid},
		{filepath.Join(base, "etc"), someUid, someGid},
		{filepath.Join(base, "etc/amazon-cloudwatch-agent.d"), someUid, someGid},
		{filepath.Join(base, "etc/amazon-cloudwatch-agent.d/default.json"), someUid, someGid},
		{filepath.Join(base, "etc/amazon-cloudwatch-agent.json"), someUid, someGid},
		{filepath.Join(base, "etc/amazon-cloudwatch-agent.toml"), someUid, someGid},
		{filepath.Join(base, "etc/common-config.toml"), someUid, someGid},
		{filepath.Join(base, "var"), someUid, someGid},
		{filepath.Join(base, "var/some-file-in-var"), someUid, someGid},
	}

	if !reflect.DeepEqual(mc.chowns, expected) {
		t.Errorf("wrong files has been changed ownership, expecting\n%v\n\n but got\n%v", expected, mc.chowns)
	}
}

func mkdir(t *testing.T, path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create dir %v: %v", path, err)
	}
}

func createFile(t *testing.T, path string, mode os.FileMode) {
	dir, _ := filepath.Split(path)
	mkdir(t, dir)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create file %v: %v", path, err)
	}
	f.Close()

	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("failed to change file mode of %v to mode %o: %v", path, mode, err)
	}
}

func createSymlink(t *testing.T, from, to string) {
	if err := os.Symlink(from, to); err != nil {
		t.Fatalf("failed to create symbolic link from %v to %v: %v", from, to, err)
	}
}
