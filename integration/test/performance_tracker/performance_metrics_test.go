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
	agentRuntime = 20 //minutes
)

func PerformanceTest(t *testing.T) {

	instanceId := test.GetInstanceId()
	log.Printf("Instance ID used for performance metrics : %s\n", instanceId)

	test.CopyFile(configPath, configOutputPath)

	test.StartAgent(configOutputPath, true)

	//let agent run before collecting performance metrics on it
	time.Sleep(agentRuntime * time.Minute)
	log.Printf("Agent has been running for : %s\n", (agentRuntime * time.Minute).String())
	test.StopAgent()
	
	//collect data
	data, err := GetPerformanceMetrics(instanceId, agentRuntime)
	if err != nil {
		log.Println("Error: " + err)
		t.Fatalf("Error: %v", err)
	}

	//------Placeholder to put data into database------//
	//useless code so data get used and compiler isn't mad
	if data == nil {
		t.Fatalf("No data")
	}
}