// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sapiserver

import (
	"os"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

const (
	SectionKeyNodeName = "node_name"
)

type NodeName struct {
}

func (n *NodeName) ApplyRule(input interface{}) (string, interface{}) {
	nodeName := os.Getenv(config.HOST_NAME)
	if nodeName == "" {
		translator.AddErrorMessages(GetCurPath(), "cannot get node_name")
		return "", nil
	}
	return SectionKeyNodeName, nodeName
}

func init() {
	RegisterRule(SectionKeyNodeName, new(NodeName))
}
