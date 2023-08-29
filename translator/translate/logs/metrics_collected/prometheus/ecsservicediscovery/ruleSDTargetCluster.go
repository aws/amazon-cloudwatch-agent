// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
)

const (
	SectionKeySDTargetCluster = "sd_target_cluster"
)

type SDTargetCluster struct {
}

func (d *SDTargetCluster) ApplyRule(input interface{}) (string, interface{}) {
	clusterName := util.GetECSClusterName(SectionKeySDTargetCluster, input.(map[string]interface{}))
	if clusterName == "" {
		translator.AddErrorMessages(GetCurPath(), "ECS Target Cluster Name is not defined")
	}
	return SectionKeySDTargetCluster, clusterName
}

func init() {
	RegisterRule(SectionKeySDTargetCluster, new(SDTargetCluster))
}
