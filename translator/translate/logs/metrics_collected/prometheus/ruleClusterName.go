// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
)

const (
	SectionKeyClusterName = "cluster_name"
)

type ClusterName struct {
}

func (c *ClusterName) ApplyRule(input interface{}) (string, interface{}) {
	clusterName := util.GetEKSClusterName(SectionKeyClusterName, input.(map[string]interface{}))

	if clusterName == "" {
		clusterName = util.GetECSClusterName(SectionKeyClusterName, input.(map[string]interface{}))
	}

	// Cluster Name is mandatory for Containerized Workloads
	if context.CurrentContext().RunInContainer() && clusterName == "" {
		translator.AddErrorMessages(GetCurPath(), "ClusterName is not defined")
	}
	return SectionKeyClusterName, clusterName
}

func init() {
	RegisterRule(SectionKeyClusterName, new(ClusterName))
}
