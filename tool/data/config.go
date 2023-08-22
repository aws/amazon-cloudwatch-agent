// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package data

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/tool/data/config"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type Config struct {
	AgentConfig   *config.AgentConfig
	MetricsConfig *config.Metrics
	LogsConfig    *config.Logs
	TracesConfig  *config.Traces
}

func (config *Config) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	if config.AgentConfig != nil {
		util.AddToMap(ctx, resultMap, config.AgentConfig)
	}
	if config.MetricsConfig != nil {
		util.AddToMap(ctx, resultMap, config.MetricsConfig)
	}
	if config.LogsConfig != nil {
		util.AddToMap(ctx, resultMap, config.LogsConfig)
	}
	if config.TracesConfig != nil {
		util.AddToMap(ctx, resultMap, config.TracesConfig)
	}

	return "", resultMap
}

func (conf *Config) AgentConf() *config.AgentConfig {
	if conf.AgentConfig == nil {
		conf.AgentConfig = new(config.AgentConfig)
	}
	return conf.AgentConfig
}
func (conf *Config) TracesConf() *config.Traces {
	if conf.TracesConfig == nil {
		conf.TracesConfig = new(config.Traces)
	}
	return conf.TracesConfig
}

func (conf *Config) MetricsConf() *config.Metrics {
	if conf.MetricsConfig == nil {
		conf.MetricsConfig = new(config.Metrics)
	}
	return conf.MetricsConfig
}

func (conf *Config) LogsConf() *config.Logs {
	if conf.LogsConfig == nil {
		conf.LogsConfig = new(config.Logs)
	}
	return conf.LogsConfig
}

func (conf *Config) SatisfiedWithCurrentConfig(context *runtime.Context) bool {
	_, resultMap := conf.ToMap(context)
	byteArray := util.SerializeResultMapToJsonByteArray(resultMap)
	fmt.Printf("Current config as follows:\n%s\n", string(byteArray))
	return util.Yes("Are you satisfied with the above config? Note: it can be manually customized after the wizard completes to add additional items.")
}
