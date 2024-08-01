// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || darwin
// +build linux darwin

package cmdutil

import (
	"log"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

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
