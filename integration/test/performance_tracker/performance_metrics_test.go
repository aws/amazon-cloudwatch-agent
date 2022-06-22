//go:build linux && integration
// +build linux,integration

package performancetest

import(
	"testing"
	"time"
	"log"
	"context"

	"github.com/aws/amazon-cloudwatch-agent/integration/test"
)

const (
	configPath = "resources/config.json"
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	agentRuntimeMinutes = 20
)

func PerformanceTest(t *testing.T) {
	agentContext := context.TODO()
	instanceId := test.GetInstanceId()
	log.Printf("Instance ID used for performance metrics : %s\n", instanceId)

	test.CopyFile(configPath, configOutputPath)

	test.StartAgent(configOutputPath, true)

	agentRunDuration := agentRuntimeMinutes * time.Minute
	//let agent run before collecting performance metrics on it
	time.Sleep(agentRunDuration)
	log.Printf("Agent has been running for : %s\n", (agentRunDuration).String())
	test.StopAgent()

	//collect data
	data, err := GetPerformanceMetrics(instanceId, agentRuntime, agentContext)
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