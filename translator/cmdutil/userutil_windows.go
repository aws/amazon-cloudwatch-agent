// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package cmdutil

import "github.com/aws/amazon-cloudwatch-agent/translator/context"

func SetupUser(u string) error {
	return nil
}

func ChangeUser(mergedJsonConfigMap map[string]interface{}) (user string, err error) {

	return "", nil
}

func VerifyCredentials(ctx *context.Context, runAsUser string) {

}

func DetectRunAsUser(mergedJsonConfigMap map[string]interface{}) (runAsUser string, err error) {
	return "", nil
}
