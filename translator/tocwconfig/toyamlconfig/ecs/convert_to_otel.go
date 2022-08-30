package ecs

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig/otelnative/translate"
)

var (
	// look for the components to determine if the agent config uses ECS, and requires
	// Container Insights configuration in the OTel YAML file
	ecsPluginIndicators = []string{"ecsdecorator"}

	// plugins by name, grouped by component (input, processor, output) that need to be operated on,
	// in order to convert the Telegraf plugins to use OTel natively
	translator = translate.AwsContainerInsightReceiver{}
)

func UsesECSConfig(plugins ...map[string]interface{}) bool {
	for _, component := range plugins {
		for key := range component {
			for _, translatable := range ecsPluginIndicators {
				if key == translatable {
					return true
				}
			}
		}
	}
	return false
}

func TranslateReceivers(inputs, processors, outputs map[string]interface{}) map[string]interface{} {
	return translator.Receivers(inputs, processors, outputs)
}

func TranslateProcessors(inputs, processors, outputs map[string]interface{}) map[string]interface{} {
	return translator.Processors(inputs, processors, outputs)
}

func TranslateExporters(inputs, processors, outputs map[string]interface{}) map[string]interface{} {
	return translator.Exporters(inputs, processors, outputs)
}

//func PluginsToComponents(plugins ...map[string]interface{}) (
//	map[config.ComponentID]config.Receiver,
//	map[config.ComponentID]config.Processor,
//	map[config.ComponentID]config.Exporter) {
//	procMap := make(map[config.ComponentID]config.Processor)
//	for _, inputs := range plugins {
//		for input := range inputs {
//			fmt.Printf("Found plugin %s\n", input)
//			// TODO: how do input configs get propagated here?
//			id := config.NewComponentID(config.Type(input))
//			hc := config.NewProcessorSettings(id)
//			procMap[id] = &hc
//		}
//	}
//	return procMap
//}
