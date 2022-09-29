// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package internal

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/security"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/util"
)

func CheckNvidiaSMIBinaryRights() error {
	if err := security.CheckFileRights(util.Default_Windows_Smi_Path); err != nil {
		return err
	}
	return nil
}
