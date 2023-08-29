// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	SectionKeySDClusterRegion = "sd_cluster_region"
)

type SDClusterRegion struct {
}

func (d *SDClusterRegion) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKeySDClusterRegion, "", input)
	if returnVal == "" {
		returnVal = ecsutil.GetECSUtilSingleton().Region
	}
	if returnVal == "" {
		translator.AddErrorMessages(GetCurPath(), "ECS Cluster Region is not defined")
	}
	return
}

func init() {
	RegisterRule(SectionKeySDClusterRegion, new(SDClusterRegion))
}
