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
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	agentRuntimeMinutes = 5 //20 mins desired but 5 mins for testing purposes
	DynamoDBDataBase = "CWAPerformanceMetrics"
)

func TestPerformance(t *testing.T) {
	configFiles := []string {
		"./resources/config10Logs.json",
		"./resources/config100Logs.json",
		"./resources/config1000Logs.json"
	}

	tpsVals := []int {
		10,
		100,
		1000
	}
	
	agentContext := context.TODO()
	instanceId := test.GetInstanceId()
	log.Printf("Instance ID used for performance metrics : %s\n", instanceId)

	//data base
	dynamoDB := InitializeTransmitterAPI(DynamoDBDataBase) //add cwa version here
	if dynamoDB == nil {
		t.Fatalf("Error: generating dynamo table")
	}

	//run tests
	for _, configPath := range configFiles {
		for _, tps := range tpsVals {
			t.Run(fmt.Sprintf("config file location: %s", configPath), func(t *testing.T) {
				test.CopyFile(configPath, configOutputPath)
	
				test.StartAgent(configOutputPath, true)
	
				agentRunDuration := agentRuntimeMinutes * time.Minute
	
				StartLogWrite(t, agentRunDuration, configPath, tps)
	
				log.Printf("Agent has been running for : %s\n", (agentRunDuration).String())
				test.StopAgent()
	
				//collect data
				data, err := GetPerformanceMetrics(instanceId, agentRuntimeMinutes, agentContext, configPath)
				//@TODO check if metrics are zero remove them and make sure there are non-zero metrics existing
				if err != nil {
					log.Println("Error: ", err)
					t.Fatalf("Error: %v", err)
				}
	
				if data == nil {
					t.Fatalf("No data")
				}
				
				_, err = dynamoDB.SendItem(data)
				if err != nil {
					t.Fatalf("Error: couldnt upload metric data to table")
				}
			})
		}
	} 
}

//StartLogWrite starts go routines to write logs to each of the logs that are monitored by CW Agent according to
//the config provided
func StartLogWrite(t *testing.T, agentRunDuration time.Duration, configFilePath string, tps int) {
	//create wait group so main test thread waits for log writing to finish before stopping agent and collecting data
	var logWaitGroup sync.WaitGroup

	logPaths := GetLogFilePaths(configFilePath)

	for i := 0; i < len(logPaths); i++ {
		logWaitGroup.Add(1)
		go func() {
			defer logWaitGroup.Done()
			WriteToLogs(t, logPaths[i], agentRunDuration, tps)
		}
	}

	//wait until writing to logs finishes
	logWaitGroup.Wait()
}

//WriteToLogs opens a file at the specified file path and writes the specified number of lines per second (tps)
//for the specified duration
func WriteToLogs(t *testing.T, filePath string, durationMinutes time.Duration, tps int) {
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Error occurred creating log file for writing: %v", err)
	}
	defer f.Close()
	defer os.Remove(filePath)

	log.Printf("Writing lines to %s with %d transactions per second", filePath, tps)

	startTime := time.Now()

	ticker := time.Ticker(1 * time.Second)
	defer ticker.Stop()

	//loop until the test duration is reached
	for {
		select {
		case <-ticker.C:
			for i := 0; i < tps; i++ {
				_, err = f.WriteString(fmt.Sprintf("%s - #%d This is a log line.\n", currTime.Format(time.StampMilli), i))
				if err != nil {
					t.Logf("Error occurred writing log line: %v", err)
				}
			}
		
		case <-time.After(durationMinutes):
			return
		}
	}
}

//GetLogFilePaths parses the cloudwatch agent config at the specified path and returns a list of the log files that the 
//agent will monitor when using that config file
func GetLogFilePaths(configPath string) []string {
	file, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Error: Could not open config file for log parsing, %v", err)
	}

	var cfgFileData map[string]interface{}
	err = json.Unmarshal(file, &cfgFileData)
	if err != nil {
		t.Fatalf("Error: Could not parse config JSON")
	}

	logFiles := cfgFileData["logs"].(map[string]interface{})["logs_collected"].(map[string]interface{})["files"].(map[string]interface{})["collect_list"].([]interface{})
	var filePaths []string
	for _, process := range logFilesList {
		filePaths = append(filePaths, process.(map[string]interface{})["file_path"].(string))
	}

	return filePaths
}
