//go:build linux && integration
// +build linux,integration

package data_collector

import(
	"testing"
	"time"
	"log"

	"github.com/aws/amazon-cloudwatch-agent/integration/test"
)

const (
	configPath = "resources/config.json"
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	agentRuntime     = 20 * time.Minute
)

func PerformanceTest(t *testing.T) {

	instanceId := test.GetInstanceId()
	log.Println("Instance ID used for performance metrics : %s", instanceId)

	test.CopyFile(configPath, configOutputPath)
	
	test.StartAgent(configOutputPath, true)

	//let agent run before collecting performance metrics on it
	time.Sleep(agentRuntime)
	log.Printf("Agent has been running for : %s", agentRuntime.String())

	//convert to int seconds for use in data collection
	runtimeSeconds := int(agentRuntime / time.Second)
	
	//collect data
	err := GetPerformanceMetrics(instanceId, runtimeSeconds)
	if (err != nil) {
		log.Println("Error: " + err)
		t.Fatalf("Error: %v", err)
	}

	test.StopAgent()
}