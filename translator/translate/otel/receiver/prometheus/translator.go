// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/file"
	"github.com/prometheus/prometheus/model/relabel"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	otelConfigParsingError = "has invalid keys: global"
	defaultTLSCaPath       = "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"
	defaultTLSCertPath     = "/etc/amazon-cloudwatch-observability-agent-ta-client-cert/client.crt"
	defaultTLSKeyPath      = "/etc/amazon-cloudwatch-observability-agent-ta-client-cert/client.key"
	ECS_SD_RESULT_FILE     = "sd_result_file"
	defaultECSSDfileName   = "/tmp/cwagent_ecs_auto_sd.yaml"
	defaultJobLabelName    = "job"
)

var ecsSDKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey, "ecs_service_discovery")

type translator struct {
	name      string
	configKey string // config key to prometheus, e.g. logs.metrics_collected.prometheus
	factory   receiver.Factory
}

func WithConfigKey(configKey string) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.configKey = configKey
		}
	}
}

func WithName(name string) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.name = name
		}
	}
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: prometheusreceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*prometheusreceiver.Config)
	configPathKey := common.ConfigKey(t.configKey, common.PrometheusConfigPathKey)

	if !conf.IsSet(configPathKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configPathKey}
	}

	configPath, _ := common.GetString(conf, configPathKey)
	processedConfigPath, err := util.GetConfigPath("prometheus.yaml", configPathKey, configPath, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to process prometheus config with given config: %w", err)
	}
	configPath = processedConfigPath.(string)
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read prometheus config from path: %w", err)
	}

	escapedContent, err := escapePrometheusConfig(content)
	if err != nil {
		return nil, fmt.Errorf("unable to escape prometheus config: %w", err)
	}
	content = escapedContent

	var stringMap map[string]interface{}
	err = yaml.Unmarshal(content, &stringMap)
	if err != nil {
		return nil, err
	}
	componentParser := confmap.NewFromStringMap(stringMap)
	if componentParser == nil {
		return nil, fmt.Errorf("unable to parse config from filename %s", configPath)
	}
	err = componentParser.Unmarshal(&cfg)
	if err != nil {
		// passed in prometheus config is in plain prometheus format and not otel wrapper
		if !strings.Contains(err.Error(), otelConfigParsingError) {
			return nil, fmt.Errorf("unable to unmarshall config to otel prometheus config from filename %s", configPath)
		}

		var promCfg prometheusreceiver.PromConfig
		err = componentParser.Unmarshal(&promCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshall config to prometheus config from filename %s", configPath)
		}
		cfg.PrometheusConfig.GlobalConfig = promCfg.GlobalConfig
		cfg.PrometheusConfig.ScrapeConfigs = promCfg.ScrapeConfigs
		cfg.PrometheusConfig.TracingConfig = promCfg.TracingConfig
	} else {
		// given prometheus config is in otel format so check if target allocator is being used
		// then add the default ca, cert, and key for TargetAllocator
		if cfg.TargetAllocator != nil && len(cfg.TargetAllocator.CollectorID) > 0 {
			cfg.TargetAllocator.TLSSetting.Config.CAFile = defaultTLSCaPath
			cfg.TargetAllocator.TLSSetting.Config.CertFile = defaultTLSCertPath
			cfg.TargetAllocator.TLSSetting.Config.KeyFile = defaultTLSKeyPath
			cfg.TargetAllocator.TLSSetting.ReloadInterval = 10 * time.Second
		}
	}
	addDefaultECSRelabelConfigs(cfg.PrometheusConfig.ScrapeConfigs, conf, t.configKey)

	return cfg, nil
}

func escapePrometheusConfig(content []byte) ([]byte, error) {
	var config map[any]any
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, err
	}
	escapeStrings(config)
	return yaml.Marshal(config)
}

func escapeStrings(node any) {
	switch n := node.(type) {
	case map[any]any:
		for k, v := range n {
			if key, ok := k.(string); ok && key == "replacement" {
				if str, ok := v.(string); ok {
					n[k] = strings.ReplaceAll(str, "$", "$$$$")
				}
			}
			escapeStrings(v)
		}
	case map[string]interface{}:
		for k, v := range n {
			if k == "replacement" {
				if str, ok := v.(string); ok {
					n[k] = strings.ReplaceAll(str, "$", "$$$$")
				}
			}
			escapeStrings(v)
		}
	case []any:
		for _, v := range n {
			escapeStrings(v)
		}
	}
}

func addDefaultECSRelabelConfigs(scrapeConfigs []*config.ScrapeConfig, conf *confmap.Conf, promConfigKey string) {
	// ECS Service Discovery Relabel Configs should only be added if enabled on ECS and configs are valid:
	if !ecsutil.GetECSUtilSingleton().IsECS() || !conf.IsSet(ecsSDKey) || len(scrapeConfigs) == 0 {
		return
	}

	ecsSdResultFileKey := common.ConfigKey(ecsSDKey, ECS_SD_RESULT_FILE)
	ecsSDFileName := defaultECSSDfileName
	if conf.IsSet(ecsSdResultFileKey) {
		if fileName, ok := conf.Get(ecsSdResultFileKey).(string); ok && fileName != "" {
			ecsSDFileName = fileName
		}
	}

	defaultRelabelConfigs := []*relabel.Config{
		{SourceLabels: model.LabelNames{"__meta_ecs_cluster_name"}, Action: relabel.Replace, TargetLabel: "ClusterName", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_cluster_name"}, Action: relabel.Replace, TargetLabel: "TaskClusterName", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_container_name"}, Action: relabel.Replace, TargetLabel: "container_name", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_task_launch_type"}, Action: relabel.Replace, TargetLabel: "LaunchType", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_task_started_by"}, Action: relabel.Replace, TargetLabel: "StartedBy", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_task_group"}, Action: relabel.Replace, TargetLabel: "TaskGroup", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_task_definition_family"}, Action: relabel.Replace, TargetLabel: "TaskDefinitionFamily", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_task_definition_revision"}, Action: relabel.Replace, TargetLabel: "TaskRevision", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_ec2_instance_type"}, Action: relabel.Replace, TargetLabel: "InstanceType", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_ec2_subnet_id"}, Action: relabel.Replace, TargetLabel: "SubnetId", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_ec2_vpc_id"}, Action: relabel.Replace, TargetLabel: "VpcId", Regex: relabel.MustNewRegexp("(.*)")},
		{SourceLabels: model.LabelNames{"__meta_ecs_source"}, Regex: relabel.MustNewRegexp("^arn:aws:ecs:.*:.*:task.*\\/(.*)$"), Action: relabel.Replace, TargetLabel: "TaskId"},
	}
	defaultRelabelConfigs = appendDockerLabelRelabelConfigs(conf, defaultRelabelConfigs)

	for _, scrapeConfig := range scrapeConfigs {
		for _, sdConfig := range scrapeConfig.ServiceDiscoveryConfigs {
			if fileSDConfig, ok := sdConfig.(*file.SDConfig); ok {
				for _, filePath := range fileSDConfig.Files {
					if filePath == ecsSDFileName {
						// Prepend defaultRelabelConfigs to customer configs for ecs_service_discovery.
						scrapeConfig.RelabelConfigs = append(defaultRelabelConfigs, scrapeConfig.RelabelConfigs...)
						break
					}
				}
			}
		}
	}
}

func appendDockerLabelRelabelConfigs(conf *confmap.Conf, defaultRelabelConfigs []*relabel.Config) []*relabel.Config {
	if hasJobLabelConfigured(conf) {
		defaultRelabelConfigs = append(defaultRelabelConfigs, &relabel.Config{SourceLabels: model.LabelNames{"__meta_ecs_container_labels_job"}, Regex: relabel.MustNewRegexp(".*"), Action: relabel.Drop})
	}
	defaultRelabelConfigs = append(defaultRelabelConfigs, &relabel.Config{Regex: relabel.MustNewRegexp("^__meta_ecs_container_labels_(.+)$"), Action: relabel.LabelMap, Replacement: prometheusreceiver.EscapedCaptureGroupOne})
	return defaultRelabelConfigs
}

// hasJobConfigured checks if job is configured in any ECS observer configuration
func hasJobLabelConfigured(conf *confmap.Conf) bool {
	// docker_label configuration
	dockerLabelKey := common.ConfigKey(ecsSDKey, "docker_label", "sd_job_name_label")
	if conf.IsSet(dockerLabelKey) {
		if jobLabelName, ok := conf.Get(dockerLabelKey).(string); ok && jobLabelName != "" && jobLabelName != defaultJobLabelName {
			return true
		}
	}

	// task_definition_list configuration
	taskDefListKey := common.ConfigKey(ecsSDKey, "task_definition_list")
	if conf.IsSet(taskDefListKey) {
		if taskDefs, ok := conf.Get(taskDefListKey).([]interface{}); ok {
			for _, td := range taskDefs {
				if tdMap, ok := td.(map[string]interface{}); ok {
					if jobName, exists := tdMap["sd_job_name"]; exists {
						if jobStr, ok := jobName.(string); ok && jobStr != "" {
							return true
						}
					}
				}
			}
		}
	}

	// service_name_list_for_tasks configuration
	serviceListKey := common.ConfigKey(ecsSDKey, "service_name_list_for_tasks")
	if conf.IsSet(serviceListKey) {
		if services, ok := conf.Get(serviceListKey).([]interface{}); ok {
			for _, svc := range services {
				if svcMap, ok := svc.(map[string]interface{}); ok {
					if jobName, exists := svcMap["sd_job_name"]; exists {
						if jobStr, ok := jobName.(string); ok && jobStr != "" {
							return true
						}
					}
				}
			}
		}
	}

	return false
}
