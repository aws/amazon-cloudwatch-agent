// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"log"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

const (
	defaultLinuxPath     = "/etc/cwagentconfig/"
	prometheusConfigName = "prometheus.yaml"
)

type ConfigPath struct {
}

func (obj *ConfigPath) ApplyRule(input interface{}) (string, interface{}) {
	configPath, err := util.GetConfigPath(prometheusConfigName, common.PrometheusConfigPathKey, defaultLinuxPath+prometheusConfigName, input)
	if err != nil {
		log.Panic(err.Error())
	}
	return common.PrometheusConfigPathKey, configPath
}

func init() {
	obj := new(ConfigPath)
	RegisterRule(common.PrometheusConfigPathKey, obj)
}
