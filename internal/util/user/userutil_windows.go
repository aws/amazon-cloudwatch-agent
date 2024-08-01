// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package user

func SetupUser(u string) error {
	return nil
}

func ChangeUser(runAsUser string) (user string, err error) {
	return "", nil
}

func DetectRunAsUser(mergedJsonConfigMap map[string]any) (runAsUser string, err error) {
	return "", nil
}
