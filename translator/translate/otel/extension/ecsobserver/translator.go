// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsobserver

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	defaultMetricsPath      = "/metrics"
	defaultPortLabel        = "ECS_PROMETHEUS_EXPORTER_PORT"
	defaultMetricsPathLabel = "ECS_PROMETHEUS_METRICS_PATH"
	defaultJobNameLabel     = "job"
)

var ecsSDKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey, common.ECSServiceDiscovery)

type translator struct {
	factory component.Factory
	name    string
}

var _ common.ComponentTranslator = (*translator)(nil)
var _ common.NameSetter = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{
		factory: ecsobserver.NewFactory(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) SetName(name string) {
	t.name = name
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(ecsSDKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: ecsSDKey}
	}

	ecsSDValue := conf.Get(ecsSDKey)
	ecsSD, ok := ecsSDValue.(map[string]interface{})
	if !ok {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: ecsSDKey}
	}

	// ECS SD Top Level Fields
	resultFile := getStringWithDefault(ecsSD, "sd_result_file", "/tmp/cwagent_ecs_auto_sd.yaml")

	clusterName := getStringWithDefault(ecsSD, "sd_target_cluster", ecsutil.GetECSUtilSingleton().Cluster)
	if clusterName == "" {
		return nil, fmt.Errorf("ECS Target Cluster Name is not defined: %s", clusterName)
	}

	clusterRegion := getStringWithDefault(ecsSD, "sd_cluster_region", ecsutil.GetECSUtilSingleton().Region)
	if clusterRegion == "" {
		return nil, fmt.Errorf("ECS Target Cluster Region is not defined: %s", clusterRegion)
	}

	refreshDuration, err := time.ParseDuration(getStringWithDefault(ecsSD, "sd_frequency", "1m"))
	if err != nil {
		return nil, fmt.Errorf("invalid refresh interval: %w", err)
	}

	cfg := &ecsobserver.Config{
		RefreshInterval: refreshDuration,
		ClusterName:     clusterName,
		ClusterRegion:   clusterRegion,
		ResultFile:      resultFile,
	}
	// Docker label based service discovery
	if dockerLabel, ok := ecsSD["docker_label"].(map[string]interface{}); ok {
		dockerConfig := ecsobserver.DockerLabelConfig{
			MetricsPathLabel: getStringWithDefault(dockerLabel, "sd_metrics_path_label", defaultMetricsPathLabel),
			PortLabel:        getStringWithDefault(dockerLabel, "sd_port_label", defaultPortLabel),
			JobNameLabel:     getStringWithDefault(dockerLabel, "sd_job_name_label", defaultJobNameLabel),
		}
		cfg.DockerLabels = []ecsobserver.DockerLabelConfig{dockerConfig} // Initialize as slice with single element
	}

	// Task definition based service discovery
	if taskDefs, ok := ecsSD["task_definition_list"].([]interface{}); ok {
		for _, td := range taskDefs {
			if tdMap, ok := td.(map[string]interface{}); ok {
				ports := parseMetricsPorts(getString(tdMap, "sd_metrics_ports"))
				taskConfig := ecsobserver.TaskDefinitionConfig{
					CommonExporterConfig: ecsobserver.CommonExporterConfig{
						JobName:      getString(tdMap, "sd_job_name"),
						MetricsPath:  getStringWithDefault(tdMap, "sd_metrics_path", defaultMetricsPath),
						MetricsPorts: convertStringPortsToInt(ports),
					},
					ArnPattern:           getString(tdMap, "sd_task_definition_arn_pattern"),
					ContainerNamePattern: getString(tdMap, "sd_container_name_pattern"),
				}
				cfg.TaskDefinitions = append(cfg.TaskDefinitions, taskConfig)
			}
		}
	}

	// Service name based service discovery
	if services, ok := ecsSD["service_name_list_for_tasks"].([]interface{}); ok {
		for _, svc := range services {
			if svcMap, ok := svc.(map[string]interface{}); ok {
				ports := parseMetricsPorts(getString(svcMap, "sd_metrics_ports"))
				serviceConfig := ecsobserver.ServiceConfig{
					CommonExporterConfig: ecsobserver.CommonExporterConfig{
						JobName:      getString(svcMap, "sd_job_name"),
						MetricsPath:  getStringWithDefault(svcMap, "sd_metrics_path", defaultMetricsPath),
						MetricsPorts: convertStringPortsToInt(ports),
					},
					NamePattern:          getString(svcMap, "sd_service_name_pattern"),
					ContainerNamePattern: getString(svcMap, "sd_container_name_pattern"),
				}
				cfg.Services = append(cfg.Services, serviceConfig)
			}
		}
	}

	return cfg, nil
}

// Add helper function to convert string ports to int
func convertStringPortsToInt(ports []string) []int {
	result := make([]int, 0, len(ports))
	for _, port := range ports {
		if p, err := strconv.Atoi(port); err == nil {
			result = append(result, p)
		}
	}
	return result
}

// Helper functions
func parseMetricsPorts(ports string) []string {
	if ports == "" {
		return nil
	}
	portList := strings.Split(ports, ";")
	var cleanPorts []string
	for _, port := range portList {
		if trimmed := strings.TrimSpace(port); trimmed != "" {
			cleanPorts = append(cleanPorts, trimmed)
		}
	}
	return cleanPorts
}

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getStringWithDefault(m map[string]interface{}, key, defaultValue string) string {
	if val := getString(m, key); val != "" {
		return val
	}
	return defaultValue
}
