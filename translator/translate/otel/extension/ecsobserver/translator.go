// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsobserver

import (
	"fmt"
	"log"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery/dockerlabel"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery/serviceendpoint"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery/taskdefinition"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type translator struct {
	factory extension.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

var (
	prometheusBaseKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)
	ecsSdBaseKey      = common.ConfigKey(prometheusBaseKey, "ecs_service_discovery")
)

func NewTranslator() common.Translator[component.Config] {
	return &translator{ecsobserver.NewFactory()}
}

func (t *translator) Type() component.Type {
	return t.factory.Type()
}

// Translate creates an ecs_observer extension config based on the fields in the
// 'ecs_service_discovery' section within the 'prometheus' section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(ecsSdBaseKey) {
		return nil, &common.MissingKeyError{Type: t.Type(), JsonKey: ecsSdBaseKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*ecsobserver.Config)
	cfg.SetIDName(common.PrometheusKey)

	_, result := new(ecsservicediscovery.ECSServiceDiscovery).ApplyRule(conf.Get(prometheusBaseKey)) // TODO: remove dependency on rule.
	c := result.(map[string]interface{})
	t.setClusterName(c, cfg)
	t.setClusterRegion(c, cfg)
	t.setResultFile(c, cfg)
	t.setRefreshInterval(c, cfg)
	t.setTaskDefinitions(c, cfg)
	t.setServices(c, cfg)
	t.setDockerLabels(c, cfg)

	if len(cfg.DockerLabels) == 0 && len(cfg.TaskDefinitions) == 0 && len(cfg.Services) == 0 {
		return nil, fmt.Errorf("neither docker label based discovery, nor task definition based discovery, nor service name based discovery is enabled")
	}
	if cfg.ClusterName == "" || cfg.ClusterRegion == "" {
		return nil, fmt.Errorf("target ECS cluster info is not correct")
	}

	return cfg, nil
}

func (t *translator) setClusterName(conf map[string]interface{}, cfg *ecsobserver.Config) {
	if clusterName, ok := conf["sd_target_cluster"]; ok {
		cfg.ClusterName = clusterName.(string)
	}
}

func (t *translator) setClusterRegion(conf map[string]interface{}, cfg *ecsobserver.Config) {
	if clusterRegion, ok := conf["sd_cluster_region"]; ok {
		cfg.ClusterRegion = clusterRegion.(string)
	}
}

func (t *translator) setResultFile(conf map[string]interface{}, cfg *ecsobserver.Config) {
	if ecsSdPath, ok := conf["sd_result_file"]; ok {
		cfg.ResultFile = ecsSdPath.(string)
	}
}

func (t *translator) setRefreshInterval(conf map[string]interface{}, cfg *ecsobserver.Config) {
	if frequency, ok := conf["sd_frequency"]; ok {
		refreshInterval, err := common.ParseDuration(frequency.(string))
		if err != nil {
			log.Printf("W! unable to extract sd_frequency from %v, using the default 60s instead", refreshInterval)
			refreshInterval, _ = common.ParseDuration("1m")
		}
		cfg.RefreshInterval = refreshInterval
	}
}

func (t *translator) setTaskDefinitions(conf map[string]interface{}, cfg *ecsobserver.Config) {
	if result, ok := conf["task_definition_list"]; ok {
		taskDefinitions, ok := result.([]interface{})
		if !ok || len(taskDefinitions) == 0 {
			return // optional section
		}

		taskDefinitionsMapped := mapSliceWithKeyMapper(taskDefinitions, map[string]string{
			"sd_task_definition_arn_pattern": "arn_pattern",
			"sd_metrics_ports":               "metrics_ports",
			"sd_container_name_pattern":      "container_name_pattern",
			"sd_metrics_path":                "metrics_path",
			"sd_job_name":                    "job_name",
		})

		c := confmap.NewFromStringMap(map[string]interface{}{
			"task_definitions": taskDefinitionsMapped, // As per https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/7f4d4425a03e7e47575211be489f912cd16ae509/extension/observer/ecsobserver/config.go#L52
		})
		err := c.Unmarshal(&cfg)
		if err != nil {
			log.Printf("W! unable to unmarshal task_definition_list due to: %v", err)
		}
	}
}

func (t *translator) setServices(conf map[string]interface{}, cfg *ecsobserver.Config) {
	if result, ok := conf["service_name_list_for_tasks"]; ok {
		services, ok := result.([]interface{})
		if !ok || len(services) == 0 {
			return // optional section
		}

		servicesMapped := mapSliceWithKeyMapper(services, map[string]string{
			"sd_service_name_pattern":   "name_pattern",
			"sd_metrics_ports":          "metrics_ports",
			"sd_container_name_pattern": "container_name_pattern",
			"sd_metrics_path":           "metrics_path",
			"sd_job_name":               "job_name",
		})

		c := confmap.NewFromStringMap(map[string]interface{}{
			"services": servicesMapped, // As per https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/7f4d4425a03e7e47575211be489f912cd16ae509/extension/observer/ecsobserver/config.go#L50
		})
		err := c.Unmarshal(&cfg)
		if err != nil {
			log.Printf("W! unable to unmarshal service_name_list_for_tasks due to: %v", err)
		}
	}
}

func (t *translator) setDockerLabels(conf map[string]interface{}, cfg *ecsobserver.Config) {
	if result, ok := conf["docker_label"]; ok {
		dockerLabel, ok := result.(map[string]interface{})
		if !ok {
			return // optional section
		}

		// ecs_observer OTel extension allows specifying multiple docker_labels whereas CWA only allows one
		dockerLabelsMapped := []map[string]interface{}{collections.WithNewKeys(dockerLabel, map[string]string{
			"sd_port_label":         "port_label",
			"sd_metrics_path_label": "metrics_path_label",
			"sd_job_name_label":     "job_name_label",
		})}

		c := confmap.NewFromStringMap(map[string]interface{}{
			"docker_labels": dockerLabelsMapped, // As per https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/7f4d4425a03e7e47575211be489f912cd16ae509/extension/observer/ecsobserver/config.go#L54
		})
		err := c.Unmarshal(&cfg)
		if err != nil {
			log.Printf("W! unable to unmarshal docker_label due to: %v", err)
		}
	}
}

func mapSliceWithKeyMapper(items []interface{}, mapper map[string]string) []map[string]interface{} {
	return collections.MapSlice(items, func(item interface{}) map[string]interface{} {
		return collections.WithNewKeys(item.(map[string]interface{}), mapper)
	})
}
