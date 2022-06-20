//go:build linux && integration
// +build linux,integration

package performancetest

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/integration/test"
)

const (
	configPath               = "resources/config.json"
	configOutputPath         = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	agentRuntimeMinutes      = 5 //20 mins desired but 5 mins for testing purposes
	DynamoDBDataBase         = "CWAPerformanceMetrics"
	logOutputPath1           = "/tmp/test1.log"
	logOutputPath2           = "/tmp/test2.log"
	transactionRatePerSecond = 10
)

func TestPerformance(t *testing.T) {
	agentContext := context.TODO()
	instanceId := test.GetInstanceId()
	log.Printf("Instance ID used for performance metrics : %s\n", instanceId)

	test.CopyFile(configPath, configOutputPath)

	test.StartAgent(configOutputPath, true)

	agentRunDuration := agentRuntimeMinutes * time.Minute

	//create wait group so main test thread waits for log writing to finish before stopping agent and collecting data
	var logWaitGroup sync.WaitGroup
	logWaitGroup.Add(2)

	//start goroutines to write to log files concurrently
	go func() {
		defer logWaitGroup.Done()
		writeToLogs(t, logOutputPath1, agentRunDuration)
	}()
	go func() {
		defer logWaitGroup.Done()
		writeToLogs(t, logOutputPath2, agentRunDuration)
	}()

	//wait until writing to logs finishes
	logWaitGroup.Wait()

	log.Printf("Agent has been running for : %s\n", (agentRunDuration).String())
	test.StopAgent()

	//collect data
	data, err := GetPerformanceMetrics(instanceId, agentRuntimeMinutes, agentContext)
	//@TODO check if metrics are zero remove them and make sure there are non-zero metrics existing
	if err != nil {
		log.Println("Error: ", err)
		t.Fatalf("Error: %v", err)
	}

	if data == nil {
		t.Fatalf("No data")
	}

	//data base
	dynamoDB := InitializeTransmitterAPI(DynamoDBDataBase) //add cwa version here
	if dynamoDB == nil {
		t.Fatalf("Error: generating dynamo table")
	}
	_, err = dynamoDB.SendItem(data)
	if err != nil {
		t.Fatalf("Error: couldnt upload metric data to table")
	}
}

func writeToLogs(t *testing.T, filePath string, durationMinutes time.Duration) {
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Error occurred creating log file for writing: %v", err)
	}
	defer f.Close()
	defer os.Remove(filePath)

	log.Printf("Writing lines to %s with %d transactions per second", filePath, transactionRatePerSecond)

	startTime := time.Now()

	//loop until the test duration is reached
	for currTime := startTime; currTime.Sub(startTime) < durationMinutes; currTime = time.Now() {

		//assume this for loop runs instantly for purposes of simple throughput calculation
		for i := 0; i < transactionRatePerSecond; i++ {
			_, err = f.WriteString(fmt.Sprintf("%s - #%d This is a log line.\n", currTime.Format(time.StampMilli), i))
			if err != nil {
				t.Logf("Error occurred writing log line: %v", err)
			}
		}

		time.Sleep(1 * time.Second)
	}
}
