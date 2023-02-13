// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emfprocessor

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/context"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/util"
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
