// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emfprocessor

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	SectionKeyLogGroupName = "log_group_name"

	K8SLogGroupNameFormat = "/aws/containerinsights/%s/prometheus"
	ECSLogGroupNameFormat = "/aws/ecs/containerinsights/%s/prometheus"
)

type LogGroupName struct {
}

func (d *LogGroupName) ApplyRule(input interface{}) (string, interface{}) {
	_, lgName := translator.DefaultCase(SectionKeyLogGroupName, "", input)
	if lgName != "" {
		return SectionKeyLogGroupName, lgName
	}

	if context.CurrentContext().RunInContainer() {
		if ecsutil.GetECSUtilSingleton().IsECS() {
			clusterName := ecsutil.GetECSUtilSingleton().Cluster
			if clusterName != "" {
				lgName = fmt.Sprintf(ECSLogGroupNameFormat, clusterName)
			}
		} else {
			clusterName := util.GetClusterNameFromEc2Tagger()
			if clusterName != "" {
				lgName = fmt.Sprintf(K8SLogGroupNameFormat, clusterName)
			}
		}
	}

	if lgName == "" {
		translator.AddErrorMessages(GetCurPath(), "Prometheus Log Group Name is not defined")
	}
	return SectionKeyLogGroupName, lgName
}

func init() {
	RegisterRule(SectionKeyLogGroupName, new(LogGroupName))
}
