package toyamlconfig

import (
	"bytes"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/csm"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/globaltags"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/files"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/files/collect_list"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/windows_events"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/windows_events/collect_list"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/ecs/cadvisor"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/ecs/ec2tagger"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/ecs/ecsdecorator"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/emf"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/kubernetes/cadvisor"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/kubernetes/ec2tagger"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/kubernetes/k8sapiserver"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/kubernetes/k8sdecorator"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery/dockerlabel"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery/serviceendpoint"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery/taskdefinition"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/emfprocessor"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/append_dimensions"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/drop_origin"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metric_decoration"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/agentInternal"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/collectd"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/cpu"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/customizedmetrics"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/disk"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/diskio"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/ethtool"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/gpu"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/mem"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/net"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/netstat"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/processes"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/procstat"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/statsd"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/swap"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/rollup_dimensions"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
	"go.opentelemetry.io/collector/config"
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

import (
	"go.opentelemetry.io/collector/service"
)

const (
	receiversKeyName = "receivers"
	exportersKeyName = "exporters"
	serviceKeyName   = "service"
	pipelinesKeyName = "pipelines"
)

func ToYamlConfig(c interface{}, fileName string) string {
	r := new(translate.Translator)
	_, val := r.ApplyRule(c)
	cn := val.(map[string]interface{})
	inputs := cn["inputs"].(map[string]interface{})
	outputs := cn["outputs"].(map[string]interface{})

	cfg := make(map[string]interface{})
	encoder := NewEncoder()

	encodeReceivers(inputs, &cfg, encoder)
	encodeExporters(outputs, &cfg, encoder)
	encodeService(inputs, outputs, &cfg, encoder)

	var buffer bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&buffer)

	if err := yamlEncoder.Encode(cfg); err != nil {
		log.Panicf("Encode to a valid YAML config fails because of %v", err)
	}
	yamlFileName := fmt.Sprintf("testdir/%v.yaml", fileName)
	err := os.WriteFile(yamlFileName, buffer.Bytes(), 0660)
	if err != nil {
		log.Fatal(err)
	}
	return buffer.String()
}

func encodeReceivers(inputs map[string]interface{}, cfg *map[string]interface{}, encoder util.Encoder) {
	receiversSection := make(map[string]interface{})

	rec := inputsToReceivers(inputs)
	receiversSection[receiversKeyName] = rec
	if err := encoder.Encode(receiversSection, &cfg); err != nil {
		log.Panicf("Encode to a valid yaml config fails because of %v", err)
	}
}

func inputsToReceivers(inputs map[string]interface{}) map[config.ComponentID]config.Receiver {
	receiverArr := make(map[config.ComponentID]config.Receiver)
	for input := range inputs {
		t := config.Type(input)
		hc := config.NewReceiverSettings(config.NewComponentID(t))
		receiverArr[config.NewComponentID(t)] = &hc
	}
	return receiverArr
}

func encodeExporters(outputs map[string]interface{}, cfg *map[string]interface{}, encoder util.Encoder) {
	exportersSection := make(map[string]interface{})
	exportersSection[exportersKeyName] = outputsToExporters(outputs)
	if err := encoder.Encode(exportersSection, &cfg); err != nil {
		log.Panicf("Encode to a valid yaml config fails because of %v", err)
	}
}

func outputsToExporters(outputs map[string]interface{}) map[config.ComponentID]config.Exporter {
	exporterArr := make(map[config.ComponentID]config.Exporter)
	for output := range outputs {
		t := config.Type(output)
		exporterSettings := config.NewExporterSettings(config.NewComponentID(t))
		exporterArr[config.NewComponentID(t)] = &exporterSettings
	}
	return exporterArr
}

func encodeService(inputs map[string]interface{}, outputs map[string]interface{}, cfg *map[string]interface{}, encoder util.Encoder) {
	serviceSection := make(map[string]interface{})
	pipelinesSection := make(map[string]interface{})
	pipelinesSection[pipelinesKeyName] = buildPipelines(outputsToExporters(outputs), inputsToReceivers(inputs))
	serviceSection[serviceKeyName] = pipelinesSection
	if err := encoder.Encode(serviceSection, &cfg); err != nil {
		log.Panicf("Encode to a valid yaml config fails because of %v", err)
	}
}

func buildPipelines(exporters map[config.ComponentID]config.Exporter, receivers map[config.ComponentID]config.Receiver) map[config.ComponentID]*service.ConfigServicePipeline {
	var exMap []config.ComponentID
	for ex := range exporters {
		exMap = append(exMap, ex)
	}
	var recMap []config.ComponentID
	for rec := range receivers {
		recMap = append(recMap, rec)
	}
	pipeline := service.ConfigServicePipeline{Exporters: exMap, Receivers: recMap}
	metricsPipeline := make(map[config.ComponentID]*service.ConfigServicePipeline)
	metricsPipeline[config.NewComponentID("metrics")] = &pipeline
	return metricsPipeline
}
