// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package windows_event_log

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGetStateFilePathGood tests getStateFilePath with good input.
func TestGetStateFilePathGood(t *testing.T) {
	fileStateFolder := filepath.Join(os.TempDir(), "CloudWatchAgentTest")
	// cleanup
	defer os.RemoveAll(fileStateFolder)
	plugin := Plugin{
		FileStateFolder: fileStateFolder,
	}
	ec := EventConfig{
		LogGroupName:  "MyGroup",
		LogStreamName: "MyStream",
		Name:          "SystemEventLog",
	}
	pathname, err := getStateFilePath(&plugin, &ec)
	t.Log(pathname)
	if err != nil {
		t.Errorf("expected nil, actual %v", err)
	}
	expected := filepath.Join(fileStateFolder,
		"Amazon_CloudWatch_WindowsEventLog_MyGroup_MyStream_SystemEventLog")
	if pathname != expected {
		t.Errorf("expected %s, actual %s", expected, pathname)
	}
	if _, err := os.Stat(fileStateFolder); os.IsNotExist(err) {
		t.Errorf("expected %s, to exist", fileStateFolder)
	}
}

// TestGetStateFilePathEscape tests getStateFilePath() with special characters.
func TestGetStateFilePathEscape(t *testing.T) {
	fileStateFolder := filepath.Join(os.TempDir(), "CloudWatchAgentTest")
	// cleanup
	defer os.RemoveAll(fileStateFolder)
	plugin := Plugin{
		FileStateFolder: fileStateFolder,
	}
	ec := EventConfig{
		LogGroupName:  "My  Group/:::",
		LogStreamName: "My::Stream//  ",
		Name:          "System  Event//Log::",
	}
	pathname, err := getStateFilePath(&plugin, &ec)
	t.Log(pathname)
	if err != nil {
		t.Errorf("expected nil, actual %v", err)
	}
	expected := filepath.Join(fileStateFolder,
		"Amazon_CloudWatch_WindowsEventLog_My__Group_____My__Stream_____System__Event__Log__")
	if pathname != expected {
		t.Errorf("expected %s, actual %s", expected, pathname)
	}
}

// TestGetStateFilePathEmpty tests getStateFilePath() with empty folder.
func TestGetStateFilePathEmpty(t *testing.T) {
	fileStateFolder := ""
	plugin := Plugin{
		FileStateFolder: fileStateFolder,
	}
	ec := EventConfig{
		LogGroupName:  "MyGroup",
		LogStreamName: "MyStream",
		Name:          "SystemEventLog",
	}
	pathname, err := getStateFilePath(&plugin, &ec)
	t.Log(pathname)
	if err == nil {
		t.Errorf("expected non-nil")
	}
}

// TestGetStateFilePathSpecialChars tests getStateFilePath() with bad folder.
func TestGetStateFilePathSpecialChars(t *testing.T) {
	fileStateFolder := "F:\\\\bin!@#$%^&*)(\\CloudWatchAgentTest"
	// cleanup
	plugin := Plugin{
		FileStateFolder: fileStateFolder,
	}
	ec := EventConfig{
		LogGroupName:  "MyGroup",
		LogStreamName: "MyStream",
		Name:          "SystemEventLog",
	}
	pathname, err := getStateFilePath(&plugin, &ec)
	t.Log(pathname)
	if err == nil {
		t.Errorf("expected non-nil")
	}
}
