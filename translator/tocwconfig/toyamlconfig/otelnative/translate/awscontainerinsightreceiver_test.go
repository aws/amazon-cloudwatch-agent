package translate

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAwsContainerInsightReceiverTranslateReceivers(t *testing.T) {
	translator := AwsContainerInsightReceiver{}
	inputs := setUpInputs(t)
	processors := setUpProcessors(t)
	outputs := setUpOutputs(t)

	result := translator.Receivers(inputs, processors, outputs)
	assert.NotEmpty(t, result)

	validateExpectedPlugins(t, result, translator, "inputs")

	receiver, ok := result[fmt.Sprintf("awscontainerinsightreceiver/%s", translator.Name())]
	assert.True(t, ok)
	receiverMap, ok := receiver.(map[string]interface{})
	assert.True(t, ok)
	collectionInterval, ok := receiverMap["collection_interval"]
	assert.True(t, ok)
	collectionIntervalStr, ok := collectionInterval.(string)
	assert.True(t, ok)
	assert.Equal(t, collectionIntervalStr, "60s")
}

func TestAwsContainerInsightReceiverTranslateProcessors(t *testing.T) {
	translator := AwsContainerInsightReceiver{}
	inputs := setUpInputs(t)
	processors := setUpProcessors(t)
	outputs := setUpOutputs(t)

	result := translator.Processors(inputs, processors, outputs)
	assert.NotEmpty(t, result)

	validateExpectedPlugins(t, result, translator, "processors")

	batch, ok := result[fmt.Sprintf("batch/%s", translator.Name())]
	assert.True(t, ok)
	batchMap, ok := batch.(map[string]interface{})
	assert.True(t, ok)
	timeout, ok := batchMap["timeout"]
	assert.True(t, ok)
	timeoutStr, ok := timeout.(string)
	assert.True(t, ok)
	assert.Equal(t, "60s", timeoutStr)
}

func TestAwsContainerInsightReceiverTranslateExporters(t *testing.T) {
	translator := AwsContainerInsightReceiver{}
	inputs := setUpInputs(t)
	processors := setUpProcessors(t)
	outputs := setUpOutputs(t)

	result := translator.Exporters(inputs, processors, outputs)
	assert.NotEmpty(t, result)

	validateExpectedPlugins(t, result, translator, "outputs")

	emf, ok := result[fmt.Sprintf("awsemf/%s", translator.Name())]
	assert.True(t, ok)
	emfPlugin, ok := emf.(map[string]interface{})
	assert.True(t, ok)
	validateEmfExporterPlugin(t, emfPlugin)
}

func TestExtractCadvisorCollectionInterval(t *testing.T) {
	m := setUpInputs(t)
	assert.Equal(t, "60s", extractCollectionInterval(m))
}

func TestExtractCadvisorCollectionIntervalMissingPlugin(t *testing.T) {
	assert.Empty(t, extractCollectionInterval(make(map[string]interface{})))
}

func TestExtractCadvisorCollectionIntervalInvalidPlugin(t *testing.T) {
	m := make(map[string]interface{})
	m["cadvisor"] = 7
	assert.Empty(t, extractCollectionInterval(m))
}

func TestExtractCadvisorCollectionIntervalInvalidInterval(t *testing.T) {
	m := make(map[string]interface{})
	cadvisor := []interface{}{
		map[string]interface{}{
			"interval":               5,
			"container_orchestrator": "ecs",
		},
	}
	m["cadvisor"] = cadvisor
	assert.Empty(t, extractCollectionInterval(m))
}

func TestPopulateDefaultEmfExporter(t *testing.T) {
	plugin, err := getDefaultEmfExporterConfig()
	assert.NoError(t, err)
	validateEmfExporterPlugin(t, plugin)
}

func TestUsesECSConfigDetectsUsage(t *testing.T) {
	translator := AwsContainerInsightReceiver{}
	inputs := setUpInputs(t)
	processors := setUpProcessors(t)
	outputs := setUpOutputs(t)

	assert.True(t, translator.RequiresTranslation(inputs, processors, outputs))
}

func TestUsesECSConfigDoesNotDetectUsage(t *testing.T) {
	m := setUpOutputs(t) // does not include the expected telegraf plugin name
	for _, plugin := range ecsPluginIndicators {
		_, ok := m[plugin]
		assert.False(t, ok)
	}
	assert.False(t, usesECSConfig(m))
}

func setUpInputs(t *testing.T) map[string]interface{} {
	t.Helper()
	m := make(map[string]interface{})
	m["cadvisor"] = []interface{}{
		map[string]interface{}{
			"interval":               "60s",
			"container_orchestrator": "ecs",
		},
	}
	m["socket_listener"] = []interface{}{
		map[string]string{
			"foo": "bar",
		},
	}

	return m
}

func setUpProcessors(t *testing.T) map[string]interface{} {
	t.Helper()
	m := make(map[string]interface{})
	m["ec2tagger"] = []interface{}{
		map[string]string{
			"foo": "bar",
		},
	}
	m["ecsdecorator"] = []interface{}{
		map[string]string{
			"foo": "bar",
		},
	}

	return m
}

func setUpOutputs(t *testing.T) map[string]interface{} {
	t.Helper()
	m := make(map[string]interface{})
	m["cloudwatchlogs"] = map[string]string{
		"foo": "bar",
	}

	return m
}

func validateExpectedPlugins(
	t *testing.T,
	pluginMap map[string]interface{},
	translator AwsContainerInsightReceiver,
	section string,
) {
	t.Helper()
	introduced, ok := translator.Introduces()[section]
	assert.True(t, ok)
	removed, ok := translator.Replaces()[section]
	assert.True(t, ok)

	for key := range pluginMap {
		for _, e := range removed {
			if key == e {
				t.Errorf("Expected %s to be removed, but it still exists", e)
			}
		}
	}
	assert.Equal(t, len(introduced), len(pluginMap))
}

func validateEmfExporterPlugin(t *testing.T, emfPlugin map[string]interface{}) {
	t.Helper()

	assert.NotNil(t, emfPlugin)
	assert.NotEmpty(t, emfPlugin)
	assert.Equal(t, "ECS/ContainerInsights", emfPlugin["namespace"])
	assert.Equal(t, "/aws/ecs/containerinsights/{ClusterName}/performance", emfPlugin["log_group_name"])
	assert.Equal(t, "instanceTelemetry/{ContainerInstanceId}", emfPlugin["log_stream_name"])

	metricDeclarations, ok := emfPlugin["metric_declarations"]
	assert.True(t, ok)
	metricDecList, ok := metricDeclarations.([]interface{})
	assert.True(t, ok)
	assert.Len(t, metricDecList, 2)
}
