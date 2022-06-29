// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package toyamlconfig

import (
	"bytes"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/encoder"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/encoder/mapstructure"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
	"go.opentelemetry.io/collector/config"
	"gopkg.in/yaml.v3"
	"log"
)

import (
	"go.opentelemetry.io/collector/service"
)

const (
	receiversKeyName = "receivers"
	exportersKeyName = "exporters"
	serviceKeyName   = "service"
	pipelinesKeyName = "pipelines"
	metricsKeyName   = "metrics"
)

func ToYamlConfig(val interface{}) (string, interface{}) {
	inputs, outputs := getInputsAndOutputs(val)
	cfg := make(map[string]interface{})
	enc := mapstructure.NewEncoder()
	receivers := encodeReceivers(inputs, &cfg, enc)
	exporters := encodeExporters(outputs, &cfg, enc)
	encodeService(receivers, exporters, &cfg, enc)

	var buffer bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&buffer)

	err := yamlEncoder.Encode(cfg)
	util.PanicIfErr("Encode to a valid YAML config fails because of", err)

	return buffer.String(), cfg
}

func encodeReceivers(inputs map[string]interface{}, cfg *map[string]interface{}, encoder encoder.Encoder) map[config.ComponentID]config.Receiver {
	receiversSection := make(map[string]interface{})

	receivers := inputsToReceivers(inputs)
	receiversSection[receiversKeyName] = receivers
	err := encoder.Encode(receiversSection, &cfg)
	util.PanicIfErr("Encode to a valid yaml config fails because of", err)
	return receivers
}

func inputsToReceivers(inputs map[string]interface{}) map[config.ComponentID]config.Receiver {
	receiverMap := make(map[config.ComponentID]config.Receiver)
	for input := range inputs {
		t := config.Type(input)
		hc := config.NewReceiverSettings(config.NewComponentID(t))
		receiverMap[config.NewComponentID(t)] = &hc
	}
	return receiverMap
}

func encodeExporters(outputs map[string]interface{}, cfg *map[string]interface{}, encoder encoder.Encoder) map[config.ComponentID]config.Exporter {
	exportersSection := make(map[string]interface{})
	exporters := outputsToExporters(outputs)
	exportersSection[exportersKeyName] = exporters
	err := encoder.Encode(exportersSection, &cfg)
	util.PanicIfErr("Encode to a valid yaml config fails because of", err)

	return exporters
}

func outputsToExporters(outputs map[string]interface{}) map[config.ComponentID]config.Exporter {
	exporterMap := make(map[config.ComponentID]config.Exporter)
	for output := range outputs {
		t := config.Type(output)
		exporterSettings := config.NewExporterSettings(config.NewComponentID(t))
		exporterMap[config.NewComponentID(t)] = &exporterSettings
	}
	return exporterMap
}

func encodeService(receivers map[config.ComponentID]config.Receiver, exporters map[config.ComponentID]config.Exporter, cfg *map[string]interface{}, encoder encoder.Encoder) {
	serviceSection := make(map[string]interface{})
	pipelinesSection := make(map[string]interface{})
	pipelinesSection[pipelinesKeyName] = buildPipelines(receivers, exporters)
	serviceSection[serviceKeyName] = pipelinesSection
	err := encoder.Encode(serviceSection, &cfg)
	util.PanicIfErr("Encode to a valid yaml config fails because of", err)
}

func buildPipelines(receivers map[config.ComponentID]config.Receiver, exporters map[config.ComponentID]config.Exporter) map[config.ComponentID]*service.ConfigServicePipeline {
	var exArray []config.ComponentID
	for ex := range exporters {
		exArray = append(exArray, ex)
	}
	var recArray []config.ComponentID
	for rec := range receivers {
		recArray = append(recArray, rec)
	}
	pipeline := service.ConfigServicePipeline{Exporters: exArray, Receivers: recArray}
	metricsPipeline := make(map[config.ComponentID]*service.ConfigServicePipeline)
	metricsPipeline[config.NewComponentID(metricsKeyName)] = &pipeline
	return metricsPipeline
}

func getInputsAndOutputs(val interface{}) (map[string]interface{}, map[string]interface{}) {
	config := val.(map[string]interface{})
	inputs, ok := config["inputs"].(map[string]interface{})
	if !ok {
		log.Panicf("E! could not extract inputs during yaml translation")
	}
	outputs, ok := config["outputs"].(map[string]interface{})
	if !ok {
		log.Panicf("E! could not extract outputs during yaml translation")
	}
	return inputs, outputs
}
