package translate

import (
	_ "embed"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/otelnative"
	"gopkg.in/yaml.v3"
)
import "fmt"

//go:embed emf_config.yml
var awsemfConfig string

// AwsContainerInsightReceiver Translating ECS to telegraf plugins converts the logs.metrics_collected.ecs
// configuration into a combination of inputs and processors:
// inputs = [cadvisor, socket_listener], processors = [ec2tagger, ecsdecorator]
// We only care about taking configuration from the cadvisor input plugin and porting them to the
// awscontainerinsightreceiver plugin
type AwsContainerInsightReceiver struct{}

func (rec AwsContainerInsightReceiver) Name() string {
	return "containerinsights"
}

func (rec AwsContainerInsightReceiver) Introduces() map[string][]string {
	return map[string][]string{
		otelnative.InputsKey:     {"awscontainerinsightreceiver"},
		otelnative.ProcessorsKey: {"batch"},
		otelnative.OutputsKey:    {"awsemf"},
	}
}

func (rec AwsContainerInsightReceiver) Replaces() map[string][]string {
	return map[string][]string{
		otelnative.InputsKey:     {"cadvisor", "socket_listener"},
		otelnative.ProcessorsKey: {"ec2tagger", "ecsdecorator"},
		otelnative.OutputsKey:    {}, // TODO: should this remove cloudwatchlogs?
	}
}

func (rec AwsContainerInsightReceiver) Receivers(in, proc, out map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	cadvisorPlugin, ok := in["cadvisor"]
	if !ok {
		return m
	}
	plugin, ok := cadvisorPlugin.([]interface{})
	if !ok {
		return m
	}
	if len(plugin) < 1 {
		return m
	}
	pluginMap, ok := plugin[0].(map[string]interface{})
	if !ok {
		return m
	}
	m["collection_interval"] = pluginMap["interval"]
	m["container_orchestrator"] = pluginMap["container_orchestrator"]

	// replace input plugins
	for _, p := range rec.Replaces()[otelnative.InputsKey] {
		delete(in, p)
	}
	in[fmt.Sprintf("awscontainerinsightreceiver/%s", rec.Name())] = m
	return in
}

func (rec AwsContainerInsightReceiver) Processors(in, proc, out map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	interval := extractCollectionInterval(in)
	if interval != "" {
		m["timeout"] = interval
	}
	// replace processor plugins
	for _, p := range rec.Replaces()[otelnative.ProcessorsKey] {
		delete(proc, p)
	}
	proc[fmt.Sprintf("batch/%s", rec.Name())] = m
	return proc
}

func (rec AwsContainerInsightReceiver) Exporters(in, proc, out map[string]interface{}) map[string]interface{} {
	// replace processor plugins
	for _, p := range rec.Replaces()[otelnative.OutputsKey] {
		delete(out, p)
	}

	m, err := getDefaultEmfExporterConfig()
	if err != nil {
		return map[string]interface{}{}
	}
	out[fmt.Sprintf("awsemf/%s", rec.Name())] = m
	return out
}

func getDefaultEmfExporterConfig() (map[string]interface{}, error) {
	m := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(awsemfConfig), m)
	if err != nil {
		return nil, err
	}

	emf, ok := m["awsemf"]
	if !ok {
		return nil, err
	}
	emfPlugin, ok := emf.(map[string]interface{})
	if !ok {
		return nil, err
	}
	return emfPlugin, nil
}

func extractCollectionInterval(inputs map[string]interface{}) string {
	cadvisorPlugin, ok := inputs["cadvisor"]
	if !ok {
		return ""
	}
	plugin, ok := cadvisorPlugin.([]interface{})
	if !ok {
		return ""
	}
	if len(plugin) < 1 {
		return ""
	}
	pluginMap, ok := plugin[0].(map[string]interface{})
	if !ok {
		return ""
	}
	interval, ok := pluginMap["interval"]
	if !ok {
		return ""
	}
	intervalStr, ok := interval.(string)
	if !ok {
		return ""
	}
	return intervalStr
}
