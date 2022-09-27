// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translate

import (
	_ "embed"
	"fmt"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/otelnative"
)

var (
	pluginName    = "cloudwatch"
	exporterName  = "awscloudwatchexporter"
	processorName = "cumulativetodeltaprocessor"
)

// Verify interface implemented
var _ otelnative.Translator = (*AwsCloudWatchExporterTranslator)(nil)

// AwsCloudWatchExporterTranslator provides the necessary YAML config contents.
type AwsCloudWatchExporterTranslator struct{}

func (et AwsCloudWatchExporterTranslator) Name() string {
	return "ace"
}

func (et AwsCloudWatchExporterTranslator) Introduces() map[string][]string {
	return map[string][]string{
		otelnative.ProcessorsKey: {processorName},
		otelnative.OutputsKey:    {exporterName},
	}
}

func (et AwsCloudWatchExporterTranslator) Replaces() map[string][]string {
	return map[string][]string{
		otelnative.OutputsKey: {pluginName},
	}
}

// RequiresTranslation checks for [[outputs.cloudwatch]].
func (et AwsCloudWatchExporterTranslator) RequiresTranslation(_, _, out map[string]interface{}) bool {
	_, ok := out[pluginName]
	return ok
}

func (et AwsCloudWatchExporterTranslator) Receivers(in, _, _ map[string]interface{}) map[string]interface{} {
	return make(map[string]interface{})

}

func (et AwsCloudWatchExporterTranslator) Processors(in, _, _ map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	m := make(map[string]interface{})
	result[fmt.Sprintf("%s/%s", processorName, et.Name())] = m
	return result
}

// Exporters takes [[outputs.cloudwatch]] out of config puts it in yaml.
func (et AwsCloudWatchExporterTranslator) Exporters(_, _, out map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	cwPlugin, ok := out[pluginName]
	if !ok {
		return result
	}
	plugin, ok := cwPlugin.([]interface{})
	if !ok {
		return result
	}
	if len(plugin) < 1 {
		return result
	}
	pluginMap, ok := plugin[0].(map[string]interface{})
	if !ok {
		return result
	}
	// Remove unecessary keys
	delete(pluginMap, "tagexclude")
	delete(pluginMap, "tagpass")
	result[fmt.Sprintf("%s/%s", exporterName, et.Name())] = pluginMap
	return result
}
