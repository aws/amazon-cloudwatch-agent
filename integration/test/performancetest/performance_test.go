//go:build linux && integration
// +build linux,integration

package performancetest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	//"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/integration/test"
)

const (
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	agentRuntimeMinutes = 5 //20 mins desired but 5 mins for testing purposes
	DynamoDBDataBase = "CWAPerformanceMetrics"
	testLogNum = "PERFORMANCE_NUMBER_OF_LOGS"
)

type LogInfo struct {
	FilePath string `json:"file_path"`
	LogGroupName string `json:"log_group_name"`
	LogStreamName string `json:"log_stream_name"`
	Timezone string `json:"timezone"`
}

func TestPerformance(t *testing.T) {
	//get number of logs for test from github action
	//logNum, err := strconv.Atoi(os.Getenv(testLogNum)) //requires a commit from Okan that updates the workflow file so the log tests will run concurrently
	// if err != nil {
	// 	t.Fatalf("Error: cannot convert test log number to integer, %v", err)
	// }
	logNum := 10 //THIS IS TEMPORARY SO CODE RUNS

	

	configFilePath, err := GenerateConfig(logNum)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	log.Printf("config generated at %s\n", configFilePath)
	defer os.Remove(configFilePath)

	tpsVals := []int {
		10,
		100,
		1000,
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
	for _, tps := range tpsVals {
		t.Run(fmt.Sprintf("TPS run: %d", tps), func(t *testing.T) {
			test.CopyFile(configFilePath, configOutputPath)

			test.StartAgent(configOutputPath, true)

			agentRunDuration := agentRuntimeMinutes * time.Minute

			err := StartLogWrite(agentRunDuration, configFilePath, tps)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}

			log.Printf("Agent has been running for : %s\n", (agentRunDuration).String())
			test.StopAgent()

			//collect data
			data, err := GetPerformanceMetrics(instanceId, agentRuntimeMinutes, agentContext, configFilePath)
			//@TODO check if metrics are zero remove them and make sure there are non-zero metrics existing
			if err != nil {
				t.Fatalf("Error: %v", err)
			}

			if data == nil {
				t.Fatalf("No data")
			}
			
			//append test metadata so we can differentiate between tests
			data, err = AppendTestMetadata(data, logNum, tps)
			if err != nil {
				t.Fatalf("Error: unable to append metadata to metric data json, %v", err)
			}

			_, err = dynamoDB.SendItem(data)
			if err != nil {
				t.Fatalf("Error: couldn't upload metric data to table, %v", err)
			}
		})
	}
}

/* GenerateConfig takes the number of logs to be monitored and applies it to a default config (at ./resources/config.json)
* it writes logs to be monitored of the form /tmp/testNUM.log where NUM is from 1 to number of logs requested to
* ./resources/configNUM.json where NUM is number of logs
* DEFAULT CONFIG MUST BE SUPPLIED WITH AT LEAST ONE LOG BEING MONITORED 
* (log being monitored will be overwritten - it is needed for json structure)
*/
func GenerateConfig(logNum int) (string, error) {
	var cfgFileData map[string]interface{}

	//use default config (for metrics, structure, etc)
	file, err := os.ReadFile("./resources/config.json")
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(file, &cfgFileData)
	if err != nil {
		return "", err
	}

	var logFiles []LogInfo

	for i := 0; i < logNum; i++ {
		logFiles = append(logFiles, LogInfo {
			FilePath: fmt.Sprintf("/tmp/test%d.log", i + 1),
			LogGroupName: "{instance_id}",
			LogStreamName: fmt.Sprintf("{instance_id}/tmp%d", i + 1),
			Timezone: "UTC",
		})
		
	}


	log.Printf("Writing config file with %d logs to ./resources/config%d.json\n", logNum, logNum)

	cfgFileData["logs"].(map[string]interface{})["logs_collected"].(map[string]interface{})["files"].(map[string]interface{})["collect_list"] = logFiles

	finalConfig, err := json.MarshalIndent(cfgFileData, "", " ")
	if err != nil {
		return "", err
	}

	configFilePath := fmt.Sprintf("./resources/config%d.json", logNum)
	f, err := os.Create(configFilePath)
	if err != nil {
		return "", err
	}

	defer f.Close()

	f.Write(finalConfig)

	return configFilePath, nil
}

//StartLogWrite starts go routines to write logs to each of the logs that are monitored by CW Agent according to
//the config provided
func StartLogWrite(agentRunDuration time.Duration, configFilePath string, tps int) (error) {
	//create wait group so main test thread waits for log writing to finish before stopping agent and collecting data
	var logWaitGroup sync.WaitGroup

	logPaths, err := GetLogFilePaths(configFilePath)
	if err != nil {
		return err
	}

	var nestedErr error
	nestedErr = nil

	for _, logPath := range logPaths {
		filePath := logPath //necessary weird golang thing
		logWaitGroup.Add(1)
		go func() {
			defer logWaitGroup.Done()
			nestedErr = WriteToLogs(filePath, agentRunDuration, tps)
		}()
	}

	//wait until writing to logs finishes
	logWaitGroup.Wait()
	return nestedErr
}

//WriteToLogs opens a file at the specified file path and writes the specified number of lines per second (tps)
//for the specified duration
func WriteToLogs(filePath string, durationMinutes time.Duration, tps int) (error) {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	defer os.Remove(filePath)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	endTimeout := time.After(durationMinutes)

	//loop until the test duration is reached
	for {
		select {
		case <-ticker.C:
			for i := 0; i < tps; i++ {
				_, err = f.WriteString(fmt.Sprintln(ticker, " - #", i, " This is a log line."))
				if err != nil {
					return err
				}
			}
		
		case <-endTimeout:
			return nil
		}
	}
}

//GetLogFilePaths parses the cloudwatch agent config at the specified path and returns a list of the log files that the 
//agent will monitor when using that config file
func GetLogFilePaths(configPath string) ([]string, error) {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfgFileData map[string]interface{}
	err = json.Unmarshal(file, &cfgFileData)
	if err != nil {
		return nil, err
	}

	logFiles := cfgFileData["logs"].(map[string]interface{})["logs_collected"].(map[string]interface{})["files"].(map[string]interface{})["collect_list"].([]interface{})
	var filePaths []string
	for _, process := range logFiles {
		filePaths = append(filePaths, process.(map[string]interface{})["file_path"].(string))
	}

	return filePaths, nil
}

//AppendTestMetadata reformats the data returned by GetPerformanceMetrics and adds metadata about the current test run
//(logs monitored and tps to those logs)
func AppendTestMetadata(data []byte, logNum, tps int) ([]byte, error) {
	var cwaData []interface{}
	err := json.Unmarshal(data, &cwaData)
	if err != nil {
		return nil, err
	}

	dataMap := make(map[string]interface{})
	dataMap["Metrics"] = cwaData
	

	dataMap["NumberOfLogsMonitored"] = logNum
	dataMap["TPS"] = tps

	outputData, err := json.MarshalIndent(dataMap, "", "  ")
	if err != nil {
		return nil, err
	}

	return outputData, nil
}