// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusreceiver

import (
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
)

var EcsRelabelConfigs = []*relabel.Config{
	{
		SourceLabels: model.LabelNames{"__meta_ecs_cluster_name"},
		TargetLabel:  "ClusterName",
		Action:       relabel.Replace,
	},
	{
		SourceLabels: model.LabelNames{"__meta_ecs_cluster_name"},
		TargetLabel:  "TaskClusterName",
		Action:       relabel.Replace,
	},
	{
		SourceLabels: model.LabelNames{"__meta_ecs_task_launch_type"},
		TargetLabel:  "LaunchType",
		Action:       relabel.Replace,
	},
	{
		SourceLabels: model.LabelNames{"__meta_ecs_task_started_by"},
		TargetLabel:  "StartedBy",
		Action:       relabel.Replace,
	},
	{
		SourceLabels: model.LabelNames{"__meta_ecs_task_group"},
		TargetLabel:  "TaskGroup",
		Action:       relabel.Replace,
	},
	{
		SourceLabels: model.LabelNames{"__meta_ecs_task_definition_family"},
		TargetLabel:  "TaskDefinitionFamily",
		Action:       relabel.Replace,
	},
	{
		SourceLabels: model.LabelNames{"__meta_ecs_task_definition_revision"},
		TargetLabel:  "TaskRevision",
		Action:       relabel.Replace,
	},
	{
		SourceLabels: model.LabelNames{"__meta_ecs_ec2_instance_type"},
		TargetLabel:  "InstanceType",
		Action:       relabel.Replace,
	},
	{
		SourceLabels: model.LabelNames{"__meta_ecs_ec2_subnet_id"},
		TargetLabel:  "SubnetId",
		Action:       relabel.Replace,
	},
	{
		SourceLabels: model.LabelNames{"__meta_ecs_ec2_vpc_id"},
		TargetLabel:  "VpcId",
		Action:       relabel.Replace,
	},
	{
		SourceLabels: model.LabelNames{"__meta_ecs_container_labels_app_x"},
		TargetLabel:  "app_x",
		Action:       relabel.Replace,
	},
	{
		Regex:       relabel.MustNewRegexp("^__meta_ecs_(.+)$"),
		Replacement: "${1}",
		Action:      relabel.LabelMap,
	},
}

var EcsMetricRelabelConfigs = []*relabel.Config{
	{
		SourceLabels: model.LabelNames{"source"},
		TargetLabel:  "TaskId",
		Regex:        relabel.MustNewRegexp("^arn:aws:ecs:.*:.*:task.*\\/(.*)$"),
		Replacement:  "${1}",
		Action:       relabel.Replace,
	},
}
