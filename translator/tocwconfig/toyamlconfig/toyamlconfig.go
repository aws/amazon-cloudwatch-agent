// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package toyamlconfig

import (
	"bytes"
	"log"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service"
	"gopkg.in/yaml.v3"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/encoder"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/encoder/mapstructure"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/otelnative"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/otelnative/translate"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
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
	TelegrafPrefix    = "telegraf_"
)

var (
	otelNativeTranslators = []otelnative.Translator{
		translate.AwsContainerInsightReceiver{},
		translate.AwsCloudWatchExporterTranslator{},
	}
)

func ToYamlConfig(val interface{}) (string, interface{}) {
	inputs := extractFromConfig(val, inputsKeyName)
	procs := extractFromConfig(val, processorsKeyName)
	outputs := extractFromConfig(val, outputsKeyName)

	nativeReceivers := make(map[string]interface{})
	nativeProcessors := make(map[string]interface{})
	nativeExporters := make(map[string]interface{})
	for _, t := range otelNativeTranslators {
		if t.RequiresTranslation(inputs, procs, outputs) {
			receivers := t.Receivers(util.CopyMap(inputs), util.CopyMap(procs), util.CopyMap(outputs))
			processors := t.Processors(util.CopyMap(inputs), util.CopyMap(procs), util.CopyMap(outputs))
			exporters := t.Exporters(util.CopyMap(inputs), util.CopyMap(procs), util.CopyMap(outputs))

			nativeReceivers = util.MergeMaps(nativeReceivers, receivers)
			nativeProcessors = util.MergeMaps(nativeProcessors, processors)
			nativeExporters = util.MergeMaps(nativeExporters, exporters)
		}
	}

	cfg := make(map[string]interface{})
	enc := mapstructure.NewEncoder()
	rec := encodeReceivers(inputs, nativeReceivers, &cfg, enc)
	proc := encodeProcessors(procs, nativeProcessors, &cfg, enc)
	ex := encodeExporters(outputs, nativeExporters, &cfg, enc)
	encodeService(rec, proc, ex, &cfg, enc)

	var buffer bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&buffer)

	err := yamlEncoder.Encode(cfg)
	util.PanicIfErr("Encode to a valid YAML config fails because of", err)
	// Delete cloudwatch output plugin section from config.
	log.Printf("I! delete cloudwatch from config")
	//delete(outputs, "cloudwatch")
	_, ok := outputs["cloudwatch"]
	if ok {
		outputs["cloudwatch"] = []struct{}{{}}
	}
	return buffer.String(), cfg
}

func encodeReceivers(inputs, nativeInputs map[string]interface{}, cfg *map[string]interface{}, encoder encoder.Encoder) map[config.ComponentID]interface{} {
	receiversSection := make(map[string]interface{})

	receivers := inputsToReceivers(inputs, nativeInputs)
	receiversSection[receiversKeyName] = receivers
	err := encoder.Encode(receiversSection, &cfg)
	util.PanicIfErr("Encode to a valid yaml config fails because of", err)
	return receivers
}

func inputsToReceivers(inputs, nativeInputs map[string]interface{}) map[config.ComponentID]interface{} {
	receiverMap := make(map[config.ComponentID]interface{})
	for key := range inputs {
		t := config.Type(TelegrafPrefix + key)
		receiverMap[config.NewComponentID(t)] = struct{}{}
	}
	for key, val := range nativeInputs {
		t := config.Type(key)
		receiverMap[config.NewComponentID(t)] = val
	}
	return receiverMap
}

func encodeProcessors(processors, nativeProcessors map[string]interface{}, cfg *map[string]interface{}, encoder encoder.Encoder) map[config.ComponentID]interface{} {
	processorsSection := make(map[string]interface{})
	p := procToProcessors(processors, nativeProcessors)
	processorsSection[processorsKeyName] = p
	err := encoder.Encode(processorsSection, &cfg)
	util.PanicIfErr("Encode to a valid yaml config fails because of", err)
	return p
}

func procToProcessors(processors, nativeProcessors map[string]interface{}) map[config.ComponentID]interface{} {
	processorMap := make(map[config.ComponentID]interface{})
	for key := range processors {
		t := config.Type(TelegrafPrefix + key)
		processorMap[config.NewComponentID(t)] = struct{}{}
	}
	for key, val := range nativeProcessors {
		t := config.Type(key)
		processorMap[config.NewComponentID(t)] = val
	}
	return processorMap
}

func encodeExporters(outputs, nativeOutputs map[string]interface{}, cfg *map[string]interface{}, encoder encoder.Encoder) map[config.ComponentID]interface{} {
	exportersSection := make(map[string]interface{})
	exporters := outputsToExporters(outputs, nativeOutputs)
	exportersSection[exportersKeyName] = exporters
	err := encoder.Encode(exportersSection, &cfg)
	util.PanicIfErr("Encode to a valid yaml config fails because of", err)

	return exporters
}

func outputsToExporters(outputs, nativeOutputs map[string]interface{}) map[config.ComponentID]interface{} {
	exporterMap := make(map[config.ComponentID]interface{})
	for key, val := range nativeOutputs {
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
		log.Printf("E! failed to extract %s from config during yaml translation", key)
		return map[string]interface{}{}
	}
	return section
}
