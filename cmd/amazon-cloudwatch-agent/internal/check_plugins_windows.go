// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package internal

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/util/security"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

func CheckNvidiaSMIBinaryRights() error {
	if err := security.CheckFileRights(util.Default_Windows_Smi_Path); err != nil {
		return err
	}
	return nil
}
