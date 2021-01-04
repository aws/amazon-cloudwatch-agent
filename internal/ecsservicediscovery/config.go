// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	AwsSdkLevelRetryCount = 3

	portSeparator = ";"
)

type ServiceNameForTasksConfig struct {
	ContainerNamePattern string `toml:"sd_container_name_pattern"`
	JobName              string `toml:"sd_job_name"`
	MetricsPath          string `toml:"sd_metrics_path"`
	MetricsPorts         string `toml:"sd_metrics_ports"`
	ServiceNamePattern   string `toml:"sd_service_name_pattern"`

	containerNameRegex *regexp.Regexp
	serviceNameRegex   *regexp.Regexp
	metricsPortList    []int
	taskArnList        []string
}

type DockerLabelConfig struct {
	JobNameLabel     string `toml:"sd_job_name_label"`
	PortLabel        string `toml:"sd_port_label"`
	MetricsPathLabel string `toml:"sd_metrics_path_label"`
}

type TaskDefinitionConfig struct {
	ContainerNamePattern string `toml:"sd_container_name_pattern"`
	JobName              string `toml:"sd_job_name"`
	MetricsPath          string `toml:"sd_metrics_path"`
	MetricsPorts         string `toml:"sd_metrics_ports"`
	TaskDefArnPattern    string `toml:"sd_task_definition_arn_pattern"`

	containerNameRegex *regexp.Regexp
	taskDefRegex       *regexp.Regexp
	metricsPortList    []int
}

func (t *TaskDefinitionConfig) String() string {
	return fmt.Sprintf("ContainerNamePattern: %v\nJobName: %v\nMetricsPath: %v\nMetricsPorts: %v\nTaskDefArnPattern: %v\n",
		t.ContainerNamePattern,
		t.JobName,
		t.MetricsPath,
		t.MetricsPorts,
		t.TaskDefArnPattern,
	)
}

func (t *TaskDefinitionConfig) init() {
	t.taskDefRegex = regexp.MustCompile(t.TaskDefArnPattern)

	if t.ContainerNamePattern != "" {
		t.containerNameRegex = regexp.MustCompile(t.ContainerNamePattern)
	}

	ports := strings.Split(t.MetricsPorts, portSeparator)
	for _, v := range ports {
		if port, err := strconv.Atoi(strings.TrimSpace(v)); err != nil || port < 0 {
			continue
		} else {
			t.metricsPortList = append(t.metricsPortList, port)
		}
	}
}

func (s *ServiceNameForTasksConfig) String() string {
	return fmt.Sprintf("ContainerNamePattern: %v\nJobName: %v\nMetricsPath: %v\nMetricsPorts: %v\nServiceNamePattern: %v\n",
		s.ContainerNamePattern,
		s.JobName,
		s.MetricsPath,
		s.MetricsPorts,
		s.ServiceNamePattern,
	)
}

func (s *ServiceNameForTasksConfig) init() {
	s.serviceNameRegex = regexp.MustCompile(s.ServiceNamePattern)

	if s.ContainerNamePattern != "" {
		s.containerNameRegex = regexp.MustCompile(s.ContainerNamePattern)
	}

	ports := strings.Split(s.MetricsPorts, portSeparator)
	for _, v := range ports {
		if port, err := strconv.Atoi(strings.TrimSpace(v)); err != nil || port < 0 {
			continue
		} else {
			s.metricsPortList = append(s.metricsPortList, port)
		}
	}
}

type ServiceDiscoveryConfig struct {
	Frequency            string                       `toml:"sd_frequency"`
	ResultFile           string                       `toml:"sd_result_file"`
	TargetCluster        string                       `toml:"sd_target_cluster"`
	TargetClusterRegion  string                       `toml:"sd_cluster_region"`
	ServiceNamesForTasks []*ServiceNameForTasksConfig `toml:"service_name_list_for_tasks"`
	DockerLabel          *DockerLabelConfig           `toml:"docker_label"`
	TaskDefinitions      []*TaskDefinitionConfig      `toml:"task_definition_list"`
}
