// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sdecorator

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"os"
)

const (
	SectionKeyHostIP = "host_ip"
)

type HostIP struct {
}

func (h *HostIP) ApplyRule(input interface{}) (string, interface{}) {
	hostIP := os.Getenv(config.HOST_IP)
	if hostIP == "" {
		translator.AddErrorMessages(GetCurPath(), "cannot get host_ip")
		return "", nil
	}
	return SectionKeyHostIP, hostIP
}

func init() {
	RegisterRule(SectionKeyHostIP, new(HostIP))
}
