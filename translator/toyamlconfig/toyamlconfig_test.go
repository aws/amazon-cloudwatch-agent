package toyamlconfig

import (
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"
)

func ReadFromFile(filename string) string {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	str := string(data)
	return strings.ReplaceAll(str, "\r\n", "\n")
}

func TestToYamlConfig(t *testing.T) {
	translator.SetTargetPlatform("linux")

	fileName := "delta_config_linux"
	jsonFileName := fmt.Sprintf("./sampleConfig/%v.json", fileName)

	var input interface{}

	err := json.Unmarshal([]byte(ReadFromFile(jsonFileName)), &input)
	assert.NoError(t, err)
	val := ToYamlConfig(input, fileName)
	expected := generateConfig(fileName)
	assert.Equal(t, expected, val)

}

func TestLinuxConfigs(t *testing.T) {
	fileNames := []string{"collectd_config_linux", "csm_only_config", "drop_origin_linux", "log_ecs_metric_only", "log_filter", "log_metric_only",
		"prometheus_config_linux", "standard_config_linux", "statsd_config"}
	translator.SetTargetPlatform("linux")
	for _, fileName := range fileNames {
		jsonFileName := fmt.Sprintf("./sampleConfig/%v.json", fileName)

		var input interface{}

		err := json.Unmarshal([]byte(ReadFromFile(jsonFileName)), &input)
		assert.NoError(t, err)
		val := ToYamlConfig(input, fileName)
		expected := generateConfig(fileName)
		assert.Equal(t, expected, val)
	}
}

func TestWindowsConfigs(t *testing.T) {

	fileNames := []string{"advanced_config_windows", "basic_config_windows", "complete_windows_config",
		"log_only_config_windows", "prometheus_config_windows",
		"standard_config_windows", "windows_eventlog_only_config"}
	translator.SetTargetPlatform("windows")
	for _, fileName := range fileNames {
		jsonFileName := fmt.Sprintf("./sampleConfig/%v.json", fileName)

		var input interface{}

		err := json.Unmarshal([]byte(ReadFromFile(jsonFileName)), &input)
		assert.NoError(t, err)
		val := ToYamlConfig(input, fileName)
		expected := generateConfig(fileName)
		assert.Equal(t, expected, val)
	}
}

func TestAdvancedWindowsConfig(t *testing.T) {

	fileNames := []string{"advanced_config_windows"}
	translator.SetTargetPlatform("windows")
	for _, fileName := range fileNames {
		jsonFileName := fmt.Sprintf("./sampleConfig/%v.json", fileName)

		var input interface{}

		err := json.Unmarshal([]byte(ReadFromFile(jsonFileName)), &input)
		assert.NoError(t, err)
		val := ToYamlConfig(input, fileName)
		expected := generateConfig(fileName)
		assert.Equal(t, expected, val)
	}
}

func generateConfig(fileName string) string {
	yamlFileName := fmt.Sprintf("testdir/%v.yaml", fileName)
	buf, err := os.ReadFile(yamlFileName)
	if err != nil {
		log.Fatalf("err %v", err)
	}
	return string(buf)
}
