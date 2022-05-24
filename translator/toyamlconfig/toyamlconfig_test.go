package toyamlconfig

import (
	"encoding/json"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"io/ioutil"
	"log"
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
	var jsonFilePath = "./sampleConfig/agentToml.json"
	var input interface{}
	translator.SetTargetPlatform("linux")
	err := json.Unmarshal([]byte(ReadFromFile(jsonFilePath)), &input)
	t.Error(err)
	val := ToYamlConfig(input)
	log.Printf(val)
}