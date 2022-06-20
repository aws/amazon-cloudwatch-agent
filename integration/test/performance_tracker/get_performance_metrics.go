package data_collector

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

//integration tests run on us-west-2
const region = "us-west-2"

func GetPerformanceMetrics(instanceId string, runtimeSeconds int) (error) {
	//load default configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return err
	}

	client := cloudwatch.NewFromConfig(cfg)

	//declare metrics you want to gather from cloudwatch agent
	ids := []string{"m1", "m2"}
	metricNames := []string{"procstat_cpu_usage", "procstat_memory_rss"}

	//give a 30 second buffer before metrics collection to allow for agent startup
	runtimeSeconds -= 30
	input, err := GenerateGetMetricInputStruct(ids, metricNames, instanceId, runtimeSeconds)
	if err != nil {
		return err
	}

	//call to cloudwatch agent API
	metrics, err := GetMetrics(context.TODO(), client, input)
	if err != nil {
		return err
	}

	//format data to json before passing output
	outputData, err := json.MarshalIndent(metrics.MetricDataResults, "", "  ")
	if err != nil {
		return err
    }
	
	//------ PASS TO DATABASE TRANSMITTER HERE------//
	//useless code so that outputData is used and compiles
	if outputData != nil {
		return nil
	}

	return nil

}
