// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ecs"
)

const (
	containerNameLabel   = "container_name"
	serviceNameLabel     = "ServiceName"
	taskFamilyLabel      = "TaskDefinitionFamily"
	taskRevisionLabel    = "TaskRevision"
	taskGroupLabel       = "TaskGroup"
	taskStartedbyLabel   = "StartedBy"
	taskLaunchTypeLabel  = "LaunchType"
	taskJobNameLabel     = "job"
	taskMetricsPathLabel = "__metrics_path__"
	taskClusterNameLabel = "TaskClusterName"
	taskIdLabel          = "TaskId"
	ec2InstanceTypeLabel = "InstanceType"
	ec2VpcIdLabel        = "VpcId"
	ec2SubnetIdLabel     = "SubnetId"

	//https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	defaultPrometheusMetricsPath = "/metrics"
)

type EC2MetaData struct {
	ContainerInstanceId string
	ECInstanceId        string
	PrivateIP           string
	InstanceType        string
	VpcId               string
	SubnetId            string
}

type DecoratedTask struct {
	Task           *ecs.Task
	TaskDefinition *ecs.TaskDefinition
	EC2Info        *EC2MetaData
	ServiceName    string

	DockerLabelBased    bool
	TaskDefinitionBased bool
}

func (t *DecoratedTask) String() string {
	return fmt.Sprintf("Task:\n\t\tTaskArn: %v\n\t\tTaskDefinitionArn: %v\n\t\tEC2Info: %v\n\t\tDockerLabelBased: %v\n\t\tTaskDefinitionBased: %v\n",
		aws.StringValue(t.Task.TaskArn),
		aws.StringValue(t.Task.TaskDefinitionArn),
		t.EC2Info,
		t.DockerLabelBased,
		t.TaskDefinitionBased,
	)
}

func addExporterLabels(labels map[string]string, labelKey string, labelValue *string) {
	if aws.StringValue(labelValue) != "" {
		labels[labelKey] = *labelValue
	}
}

// Get the private ip of the decorated task.
// Return "" when fail to get the private ip
func (t *DecoratedTask) getPrivateIp() string {
	networkMode := aws.StringValue(t.TaskDefinition.NetworkMode)
	if networkMode == ecs.NetworkModeNone {
		return ""
	}

	// AWSVPC: Get Private IP from tasks->attachments (ElasticNetworkInterface -> privateIPv4Address)
	if networkMode == ecs.NetworkModeAwsvpc {
		for _, v := range t.Task.Attachments {
			if aws.StringValue(v.Type) == "ElasticNetworkInterface" {
				for _, d := range v.Details {
					if aws.StringValue(d.Name) == "privateIPv4Address" {
						return aws.StringValue(d.Value)
					}
				}
			}
		}
	}

	if t.EC2Info != nil {
		return t.EC2Info.PrivateIP
	}
	return ""
}

func (t *DecoratedTask) getPrometheusExporterPort(configuredPort int64, c *ecs.ContainerDefinition) int64 {
	var mappedPort int64
	networkMode := aws.StringValue(t.TaskDefinition.NetworkMode)
	if networkMode == ecs.NetworkModeNone {
		// for network type: none, skipped directly
		return 0
	}

	if networkMode == ecs.NetworkModeAwsvpc || networkMode == ecs.NetworkModeHost {
		// for network type: awsvpc or host, get the mapped port from: taskDefinition->containerDefinitions->portMappings
		for _, v := range c.PortMappings {
			if aws.Int64Value(v.ContainerPort) == configuredPort {
				mappedPort = aws.Int64Value(v.HostPort)
			}
		}
	} else if networkMode == ecs.NetworkModeBridge || networkMode == "" {
		// for network type: bridge, get the mapped port from: task->containers->networkBindings
		containerName := aws.StringValue(c.Name)
		for _, tc := range t.Task.Containers {
			if containerName == aws.StringValue(tc.Name) {
				for _, v := range tc.NetworkBindings {
					if aws.Int64Value(v.ContainerPort) == configuredPort {
						mappedPort = aws.Int64Value(v.HostPort)
					}
				}
			}
		}
	}
	return mappedPort
}

func (t *DecoratedTask) generatePrometheusTarget(
	dockerLabelReg *regexp.Regexp,
	c *ecs.ContainerDefinition,
	ip string,
	mappedPort int64,
	metricsPath string,
	customizedJobName string) *PrometheusTarget {

	labels := make(map[string]string)
	addExporterLabels(labels, containerNameLabel, c.Name)
	addExporterLabels(labels, taskFamilyLabel, t.TaskDefinition.Family)
	revisionStr := fmt.Sprintf("%d", *t.TaskDefinition.Revision)
	addExporterLabels(labels, taskRevisionLabel, &revisionStr)
	addExporterLabels(labels, taskGroupLabel, t.Task.Group)
	addExporterLabels(labels, taskStartedbyLabel, t.Task.StartedBy)
	addExporterLabels(labels, taskLaunchTypeLabel, t.Task.LaunchType)

	if arn, err := arn.Parse(*t.Task.TaskArn); err == nil {
		// ARN formats: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-account-settings.html#ecs-resource-ids
		splitResource := strings.Split(arn.Resource, "/")[1:]
		if len(splitResource) == 1 {
			// Old ARN format
			taskId := splitResource[0]
			addExporterLabels(labels, taskIdLabel, &taskId)
		} else if len(splitResource) == 2 {
			// New ARN format
			clusterName := splitResource[0]
			taskId := splitResource[1]
			addExporterLabels(labels, taskClusterNameLabel, &clusterName)
			addExporterLabels(labels, taskIdLabel, &taskId)
		}
	}

	if t.EC2Info != nil {
		addExporterLabels(labels, ec2InstanceTypeLabel, &t.EC2Info.InstanceType)
		addExporterLabels(labels, ec2VpcIdLabel, &t.EC2Info.VpcId)
		addExporterLabels(labels, ec2SubnetIdLabel, &t.EC2Info.SubnetId)
	}

	addExporterLabels(labels, taskMetricsPathLabel, &metricsPath)
	for k, v := range c.DockerLabels {
		if dockerLabelReg.MatchString(k) {
			addExporterLabels(labels, k, v)
		}
	}
	// handle customized job label at last, so the conflict job docker label is overridden
	addExporterLabels(labels, taskJobNameLabel, &customizedJobName)

	return &PrometheusTarget{
		Targets: []string{fmt.Sprintf("%s:%d", ip, mappedPort)},
		Labels:  labels,
	}
}

func (t *DecoratedTask) exportDockerLabelBasedTarget(config *ServiceDiscoveryConfig,
	dockerLabelReg *regexp.Regexp,
	ip string,
	c *ecs.ContainerDefinition,
	targets map[string]*PrometheusTarget) {

	if !t.DockerLabelBased {
		return
	}

	configuredPortStr, ok := c.DockerLabels[config.DockerLabel.PortLabel]
	if !ok {
		// skip the container without matching sd_port_label
		return
	}

	var exporterPort int64
	if port, err := strconv.Atoi(aws.StringValue(configuredPortStr)); err != nil || port < 0 {
		// an invalid port definition.
		return
	} else {
		exporterPort = int64(port)
	}
	mappedPort := t.getPrometheusExporterPort(exporterPort, c)
	if mappedPort == 0 {
		return
	}

	metricsPath := defaultPrometheusMetricsPath
	metricsPathLabel := ""
	if v, ok := c.DockerLabels[config.DockerLabel.MetricsPathLabel]; ok {
		metricsPath = *v
		metricsPathLabel = *v
	}
	targetKey := fmt.Sprintf("%s:%d%s", ip, mappedPort, metricsPath)
	if _, ok := targets[targetKey]; ok {
		return
	}

	customizedJobName := ""
	if _, ok := c.DockerLabels[config.DockerLabel.JobNameLabel]; ok {
		customizedJobName = *c.DockerLabels[config.DockerLabel.JobNameLabel]
	}

	targets[targetKey] = t.generatePrometheusTarget(dockerLabelReg, c, ip, mappedPort, metricsPathLabel, customizedJobName)
}

func (t *DecoratedTask) exportTaskDefinitionBasedTarget(config *ServiceDiscoveryConfig,
	dockerLabelReg *regexp.Regexp,
	ip string,
	c *ecs.ContainerDefinition,
	targets map[string]*PrometheusTarget) {

	if !t.TaskDefinitionBased {
		return
	}

	for _, v := range config.TaskDefinitions {
		// skip if task def regex mismatch
		if !v.taskDefRegex.MatchString(*t.Task.TaskDefinitionArn) {
			continue
		}

		// skip if there is container name regex pattern configured and container name mismatch
		if v.ContainerNamePattern != "" && !v.containerNameRegex.MatchString(*c.Name) {
			continue
		}

		for _, port := range v.metricsPortList {
			mappedPort := t.getPrometheusExporterPort(int64(port), c)
			if mappedPort == 0 {
				continue
			}

			metricsPath := defaultPrometheusMetricsPath
			if v.MetricsPath != "" {
				metricsPath = v.MetricsPath
			}
			targetKey := fmt.Sprintf("%s:%d%s", ip, mappedPort, metricsPath)

			if _, ok := targets[targetKey]; ok {
				continue
			}

			targets[targetKey] = t.generatePrometheusTarget(dockerLabelReg, c, ip, mappedPort, v.MetricsPath, v.JobName)
		}

	}
}

func (t *DecoratedTask) exportServiceEndpointBasedTarget(config *ServiceDiscoveryConfig,
	dockerLabelReg *regexp.Regexp,
	ip string,
	c *ecs.ContainerDefinition,
	targets map[string]*PrometheusTarget) {

	if t.ServiceName == "" {
		return
	}

	for _, v := range config.ServiceNamesForTasks {
		// skip if service name regex mismatch
		if !v.serviceNameRegex.MatchString(t.ServiceName) {
			continue
		}

		if v.ContainerNamePattern != "" && !v.containerNameRegex.MatchString(*c.Name) {
			continue
		}

		for _, port := range v.metricsPortList {
			mappedPort := t.getPrometheusExporterPort(int64(port), c)
			if mappedPort == 0 {
				continue
			}

			metricsPath := defaultPrometheusMetricsPath
			if v.MetricsPath != "" {
				metricsPath = v.MetricsPath
			}
			targetKey := fmt.Sprintf("%s:%d%s", ip, mappedPort, metricsPath)

			if _, ok := targets[targetKey]; ok {
				continue
			}

			prometheusTarget := t.generatePrometheusTarget(dockerLabelReg, c, ip, mappedPort, v.MetricsPath, v.JobName)
			addExporterLabels(prometheusTarget.Labels, serviceNameLabel, &t.ServiceName)
			targets[targetKey] = prometheusTarget
		}
	}

}

func (t *DecoratedTask) ExporterInformation(config *ServiceDiscoveryConfig, dockerLabelRegex *regexp.Regexp, targets map[string]*PrometheusTarget) {
	ip := t.getPrivateIp()
	if ip == "" {
		return
	}
	for _, c := range t.TaskDefinition.ContainerDefinitions {
		t.exportServiceEndpointBasedTarget(config, dockerLabelRegex, ip, c, targets)
		t.exportDockerLabelBasedTarget(config, dockerLabelRegex, ip, c, targets)
		t.exportTaskDefinitionBasedTarget(config, dockerLabelRegex, ip, c, targets)
	}
}
