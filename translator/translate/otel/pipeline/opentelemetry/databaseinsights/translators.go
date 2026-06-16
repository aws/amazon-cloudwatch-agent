// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package databaseinsights

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// ottlSafeRegex restricts values interpolated into OTTL statements to safe
// characters, preventing injection of OTTL syntax or string escape sequences.
var ottlSafeRegex = regexp.MustCompile(`^[a-zA-Z0-9._@/:\-]+$`)

var (
	dbiMysqlKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.DatabaseInsightsKey, common.MySQLKey)
)

type dbiInstanceConfig struct {
	endpoint     string
	username     string
	passfile     string
	caFile       string
	instanceName string
	logFilePath  string
	isLocalhost  bool
}

func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	if conf == nil || !conf.IsSet(common.DatabaseInsightsConfigKey) {
		return translators
	}

	pgInstances := parseDbiPostgresqlInstances(conf)
	for i, cfg := range pgInstances {
		translators.Set(&dbiTranslator{pipelineType: dbiMetrics, instanceIndex: i, cfg: cfg})
		translators.Set(&dbiTranslator{pipelineType: dbiLogToMetrics, instanceIndex: i, cfg: cfg})
		translators.Set(&dbiTranslator{pipelineType: dbiRawEvents, instanceIndex: i, cfg: cfg})
		if cfg.logFilePath != "" {
			translators.Set(&dbiTranslator{pipelineType: dbiServerLogs, instanceIndex: i, cfg: cfg})
		}
	}

	mysqlInstances := parseDbiMysqlInstances(conf)
	for i, cfg := range mysqlInstances {
		translators.Set(&dbiMysqlTranslator{pipelineType: dbiMetrics, instanceIndex: i, cfg: cfg})
		translators.Set(&dbiMysqlTranslator{pipelineType: dbiLogToMetrics, instanceIndex: i, cfg: cfg})
		translators.Set(&dbiMysqlTranslator{pipelineType: dbiRawEvents, instanceIndex: i, cfg: cfg})
		if cfg.logFilePath != "" {
			translators.Set(&dbiMysqlTranslator{pipelineType: dbiServerLogs, instanceIndex: i, cfg: cfg})
		}
	}

	return translators
}

type pgRawInstance struct {
	Endpoint     string `mapstructure:"endpoint"`
	Username     string `mapstructure:"username"`
	PasswordFile string `mapstructure:"password_file"`
	CAFile       string `mapstructure:"ca_file"`
	InstanceName string `mapstructure:"instance_name"`
	Logs         struct {
		FilePath string `mapstructure:"file_path"`
	} `mapstructure:"logs"`
}

func parseDbiPostgresqlInstances(conf *confmap.Conf) []dbiInstanceConfig {
	arr, _ := conf.Get(common.DatabaseInsightsPostgresKey).([]any)
	var raw []pgRawInstance
	if err := mapstructure.Decode(arr, &raw); err != nil {
		return nil
	}
	instances := make([]dbiInstanceConfig, 0, len(raw))
	for _, r := range raw {
		instances = append(instances, dbiInstanceConfig{
			endpoint:     r.Endpoint,
			username:     r.Username,
			passfile:     r.PasswordFile,
			caFile:       r.CAFile,
			instanceName: r.InstanceName,
			logFilePath:  r.Logs.FilePath,
			isLocalhost:  isLocalhostEndpoint(r.Endpoint),
		})
	}
	return instances
}

func validateOttlSafe(field, value string) error {
	if value == "" {
		return nil
	}
	if !ottlSafeRegex.MatchString(value) {
		return fmt.Errorf("database_insights %s contains invalid characters: %q (must match %s)", field, value, ottlSafeRegex.String())
	}
	return nil
}

type mysqlRawInstance struct {
	Endpoint     string `mapstructure:"endpoint"`
	Username     string `mapstructure:"username"`
	PasswordFile string `mapstructure:"password_file"`
	InstanceName string `mapstructure:"instance_name"`
	TLS          struct {
		CAFile string `mapstructure:"ca_file"`
	} `mapstructure:"tls"`
	Logs struct {
		FilePath string `mapstructure:"file_path"`
	} `mapstructure:"logs"`
}

func parseDbiMysqlInstances(conf *confmap.Conf) []dbiInstanceConfig {
	arr, _ := conf.Get(dbiMysqlKey).([]any)
	var raw []mysqlRawInstance
	if err := mapstructure.Decode(arr, &raw); err != nil {
		return nil
	}
	instances := make([]dbiInstanceConfig, 0, len(raw))
	for _, r := range raw {
		instances = append(instances, dbiInstanceConfig{
			endpoint:     r.Endpoint,
			username:     r.Username,
			passfile:     r.PasswordFile,
			caFile:       r.TLS.CAFile,
			instanceName: r.InstanceName,
			logFilePath:  r.Logs.FilePath,
			isLocalhost:  isLocalhostEndpoint(r.Endpoint),
		})
	}
	return instances
}

func isLocalhostEndpoint(endpoint string) bool {
	return strings.HasPrefix(endpoint, "localhost") ||
		strings.HasPrefix(endpoint, "127.0.0.1") ||
		strings.HasPrefix(endpoint, "[::1]") ||
		strings.HasPrefix(endpoint, "::1")
}
