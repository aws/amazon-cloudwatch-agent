// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsdecorator

import (
	"os"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
)

const (
	SectionKeyHostIP = "host_ip"
)

type HostIP struct{}

func (h *HostIP) ApplyRule(input interface{}) (string, interface{}) {
	if hostIP := os.Getenv(config.HOST_IP); hostIP != "" {
		return SectionKeyHostIP, hostIP
	}
	return SectionKeyHostIP, ec2util.GetEC2UtilSingleton().PrivateIP
}

func init() {
	RegisterRule(SectionKeyHostIP, new(HostIP))
}
