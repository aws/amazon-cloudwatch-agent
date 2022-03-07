// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package csm

import (
	"fmt"
	"net"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

func computeIPv4LoopbackAddressFromPort(port interface{}) string {
	return fmt.Sprintf("udp4://127.0.0.1:%v", port)
}

func computeIPv6LoopbackAddressFromPort(port interface{}) string {
	return fmt.Sprintf("udp6://[::1]:%v", port)
}

func checkCsmSocketListener(listenAddress string) bool {
	splitAddress := strings.SplitN(listenAddress, "://", 2)
	if len(splitAddress) != 2 {
		translator.AddErrorMessages(GetCurPath(), fmt.Sprintf("Invalid service address: %s", listenAddress))
		return false
	}

	// Does the local platform support listening on the specified address (protocol included)?
	conn, err := net.ListenPacket(splitAddress[0], splitAddress[1])
	if conn != nil {
		conn.Close()
	}

	if err != nil {
		translator.AddInfoMessages(GetCurPath(), fmt.Sprintf("Failed binding listener socket implied by address %s, with error %v", listenAddress, err))
		return false
	}

	return true
}
