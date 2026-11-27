// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dpdk

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type SocketPath struct {
}

const SectionKey_SocketPath = "socket_path"

// Default path of the DPDK v2 telemetry socket, as documented in
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/monitoring-network-performance-ena.html#network-performance-metrics-dpdk
const defaultSocketPath = "/var/run/dpdk/rte/dpdk_telemetry.v2"

func (obj *SocketPath) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_SocketPath, defaultSocketPath, input)
	return
}

func init() {
	obj := new(SocketPath)
	RegisterRule(SectionKey_SocketPath, obj)
}
