// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package toyamlconfig

import (
	"bytes"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/ecs"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/encoder"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/encoder/mapstructure"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service"
	"gopkg.in/yaml.v3"
	"log"
)

const (
	receiversKeyName  = "receivers"
	processorsKeyName = "processors"
	exportersKeyName  = "exporters"
	serviceKeyName    = "service"
	pipelinesKeyName  = "pipelines"
	metricsKeyName    = "metrics"
	inputsKeyName     = "inputs"
	outputsKeyName    = "outputs"
)

func ToYamlConfig(val interface{}) (string, interface{}) {
	inputs := extractFromConfig(val, inputsKeyName)
	procs := extractFromConfig(val, processorsKeyName)
	outputs := extractFromConfig(val, outputsKeyName)

	if ecs.UsesECSConfig(inputs, procs, outputs) {
		log.Println("Config uses ECS. Include container insights configurations")
		newInputs := ecs.TranslateReceivers(copyMap(inputs), copyMap(procs), copyMap(outputs))
		newProcs := ecs.TranslateProcessors(copyMap(inputs), copyMap(procs), copyMap(outputs))
		newOutputs := ecs.TranslateExporters(copyMap(inputs), copyMap(procs), copyMap(outputs))

		inputs = newInputs
		procs = newProcs
		outputs = newOutputs
	}

	cfg := make(map[string]interface{})
	enc := mapstructure.NewEncoder()
	rec := encodeReceivers(inputs, &cfg, enc)
	proc := encodeProcessors(procs, &cfg, enc)
	ex := encodeExporters(outputs, &cfg, enc)
	encodeService(rec, proc, ex, &cfg, enc)

	var buffer bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&buffer)

	err := yamlEncoder.Encode(cfg)
	util.PanicIfErr("Encode to a valid YAML config fails because of", err)

	return buffer.String(), cfg
}

func encodeReceivers(inputs map[string]interface{}, cfg *map[string]interface{}, encoder encoder.Encoder) map[config.ComponentID]interface{} {
	receiversSection := make(map[string]interface{})

	receivers := inputsToReceivers(inputs)
	receiversSection[receiversKeyName] = receivers
	err := encoder.Encode(receiversSection, &cfg)
	util.PanicIfErr("Encode to a valid yaml config fails because of", err)
	return receivers
}

func inputsToReceivers(inputs map[string]interface{}) map[config.ComponentID]interface{} {
	receiverMap := make(map[config.ComponentID]interface{})
	for key, val := range inputs {
		t := config.Type(key)
		receiverMap[config.NewComponentID(t)] = val
	}
	return receiverMap
}

func encodeProcessors(processors map[string]interface{}, cfg *map[string]interface{}, encoder encoder.Encoder) map[config.ComponentID]interface{} {
	processorsSection := make(map[string]interface{})
	p := procToProcessors(processors)
	processorsSection[processorsKeyName] = p
	err := encoder.Encode(processorsSection, &cfg)
	util.PanicIfErr("Encode to a valid yaml config fails because of", err)
	return p
}

func procToProcessors(processors map[string]interface{}) map[config.ComponentID]interface{} {
	processorMap := make(map[config.ComponentID]interface{})
	for key, val := range processors {
		t := config.Type(key)
		processorMap[config.NewComponentID(t)] = val
	}
	return processorMap
}

func encodeExporters(outputs map[string]interface{}, cfg *map[string]interface{}, encoder encoder.Encoder) map[config.ComponentID]interface{} {
	exportersSection := make(map[string]interface{})
	exporters := outputsToExporters(outputs)
	exportersSection[exportersKeyName] = exporters
	err := encoder.Encode(exportersSection, &cfg)
	util.PanicIfErr("Encode to a valid yaml config fails because of", err)

	return exporters
}

func outputsToExporters(outputs map[string]interface{}) map[config.ComponentID]interface{} {
	exporterMap := make(map[config.ComponentID]interface{})
	for key, val := range outputs {
		t := config.Type(key)
		exporterMap[config.NewComponentID(t)] = val
	}
	return exporterMap
}

func encodeService(receivers map[config.ComponentID]interface{}, processors map[config.ComponentID]interface{}, exporters map[config.ComponentID]interface{}, cfg *map[string]interface{}, encoder encoder.Encoder) {
	serviceSection := make(map[string]interface{})
	pipelinesSection := make(map[string]interface{})
	pipelinesSection[pipelinesKeyName] = buildPipelines(receivers, processors, exporters)
	serviceSection[serviceKeyName] = pipelinesSection
	err := encoder.Encode(serviceSection, &cfg)
	util.PanicIfErr("Encode to a valid yaml config fails because of", err)
}

func buildPipelines(receiverMap map[config.ComponentID]interface{}, processorMap map[config.ComponentID]interface{}, exporterMap map[config.ComponentID]interface{}) map[config.ComponentID]*service.ConfigServicePipeline {
	var exporters []config.ComponentID
	for ex := range exporterMap {
		exporters = append(exporters, ex)
	}
	var procs []config.ComponentID
	for proc := range processorMap {
		procs = append(procs, proc)
	}
	var receivers []config.ComponentID
	for rec := range receiverMap {
		receivers = append(receivers, rec)
	}
	pipeline := service.ConfigServicePipeline{Receivers: receivers, Processors: procs, Exporters: exporters}
	metricsPipeline := make(map[config.ComponentID]*service.ConfigServicePipeline)
	metricsPipeline[config.NewComponentID(metricsKeyName)] = &pipeline
	return metricsPipeline
}

func extractFromConfig(cfg interface{}, key string) map[string]interface{} {
	c, ok := cfg.(map[string]interface{})
	if !ok {
		log.Panic("E! could not extract from invalid configuration")
	}

	section, ok := c[key].(map[string]interface{})
	if !ok {
		log.Panicf("E! failed to extract %s from config during yaml translation", key)
	}
	return section
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	dupe := make(map[string]interface{})
	for k, v := range m {
		dupe[k] = v
	}
	return dupe
}
