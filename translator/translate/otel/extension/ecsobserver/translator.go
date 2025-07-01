// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsobserver

import (
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
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

	ecsSD := common.GetIndexedMap(conf, ecsSDKey, -1)
	if ecsSD == nil {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: ecsSDKey}
	}

	requiredFields := []string{"sd_target_cluster", "sd_cluster_region", "sd_result_file"}
	for _, field := range requiredFields {
		if _, ok := ecsSD[field]; !ok {
			return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: field}
		}
	}

	// Base config with mandatory fields
	cfg := &Config{
		RefreshInterval: getStringWithDefault(ecsSD, "sd_frequency", "10s"),
		ClusterName:     getString(ecsSD, "sd_target_cluster"),
		ClusterRegion:   getString(ecsSD, "sd_cluster_region"),
		ResultFile:      getString(ecsSD, "sd_result_file"),
	}

	// Docker label based service discovery
	if dockerLabel, ok := ecsSD["docker_label"].(map[string]interface{}); ok {
		dockerConfig := &DockerLabelConfig{
			PortLabel:        getStringWithDefault(dockerLabel, "sd_port_label", defaultPortLabel),
			MetricsPathLabel: getStringWithDefault(dockerLabel, "sd_metrics_path_label", defaultMetricsPathLabel),
			JobNameLabel:     getStringWithDefault(dockerLabel, "sd_job_name_label", defaultJobNameLabel),
		}
		cfg.DockerLabels = []*DockerLabelConfig{dockerConfig}
	}

	// Task definition based service discovery
	if taskDefs, ok := ecsSD["task_definition_list"].([]interface{}); ok {
		for _, td := range taskDefs {
			if tdMap, ok := td.(map[string]interface{}); ok {
				taskConfig := TaskDefinitionConfig{
					ArnPattern:           getString(tdMap, "sd_task_definition_arn_pattern"),
					MetricsPorts:         parseMetricsPorts(getString(tdMap, "sd_metrics_ports")),
					MetricsPath:          getStringWithDefault(tdMap, "sd_metrics_path", defaultMetricsPath),
					ContainerNamePattern: getStringWithDefault(tdMap, "sd_container_name_pattern", ""),
					JobName:              getString(tdMap, "sd_job_name"),
				}
				cfg.TaskDefinitions = append(cfg.TaskDefinitions, taskConfig)
			}
		}
	}

	// Service name based service discovery
	if services, ok := ecsSD["service_name_list_for_tasks"].([]interface{}); ok {
		for _, svc := range services {
			if svcMap, ok := svc.(map[string]interface{}); ok {
				serviceConfig := ServiceConfig{
					NamePattern:          getString(svcMap, "sd_service_name_pattern"),
					MetricsPorts:         parseMetricsPorts(getString(svcMap, "sd_metrics_ports")),
					MetricsPath:          getStringWithDefault(svcMap, "sd_metrics_path", defaultMetricsPath),
					ContainerNamePattern: getStringWithDefault(svcMap, "sd_container_name_pattern", ""),
					JobName:              getString(svcMap, "sd_job_name"),
				}
				cfg.Services = append(cfg.Services, serviceConfig)
			}
		}
	}

	return cfg, nil
}

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

// Config represents the configuration for the ECS observer extension
type Config struct {
	RefreshInterval string                 `mapstructure:"refresh_interval"` // How often to refresh service discovery
	ClusterName     string                 `mapstructure:"cluster_name"`     // Target ECS cluster name
	ClusterRegion   string                 `mapstructure:"cluster_region"`   // AWS region of the ECS cluster
	ResultFile      string                 `mapstructure:"result_file"`      // Path to write discovery results
	DockerLabels    []*DockerLabelConfig   `mapstructure:"docker_labels,omitempty"`
	TaskDefinitions []TaskDefinitionConfig `mapstructure:"task_definitions,omitempty"`
	Services        []ServiceConfig        `mapstructure:"services,omitempty"`
}

// DockerLabelConfig represents the docker label based service discovery configuration
type DockerLabelConfig struct {
	PortLabel        string `mapstructure:"port_label"`
	MetricsPathLabel string `mapstructure:"metrics_path_label,omitempty"`
	JobNameLabel     string `mapstructure:"job_name_label,omitempty"`
}

// TaskDefinitionConfig represents the task definition based service discovery configuration
type TaskDefinitionConfig struct {
	ArnPattern           string   `mapstructure:"arn_pattern"`
	ContainerNamePattern string   `mapstructure:"container_name_pattern,omitempty"`
	MetricsPorts         []string `mapstructure:"metrics_ports"`
	MetricsPath          string   `mapstructure:"metrics_path,omitempty"`
	JobName              string   `mapstructure:"job_name,omitempty"`
}

// ServiceConfig represents the service name based service discovery configuration
type ServiceConfig struct {
	NamePattern          string   `mapstructure:"name_pattern"`
	ContainerNamePattern string   `mapstructure:"container_name_pattern,omitempty"`
	MetricsPorts         []string `mapstructure:"metrics_ports"`
	MetricsPath          string   `mapstructure:"metrics_path,omitempty"`
	JobName              string   `mapstructure:"job_name,omitempty"`
}