package performancetest

import (
	"time"
	"errors"
	"context"
	"encoding/json"
	"os"
	"fmt"
	
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

const (
	Namespace = "CWAgent"
	DimensionName = "InstanceId"
	Stat = "Average"
	Period = 30
	configPath = "./resources/config.json"
)

/*
 * GetConfigMetrics parses the cloudwatch agent config and returns the associated 
 * metrics that the cloudwatch agent is measuring on itself
*/ 
func GetConfigMetrics() ([]string, []string, error) {
	//get metric measurements from config file
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, err
	}

	var cfgFileData map[string]interface{}
	err = json.Unmarshal(file, &cfgFileData)
	if err != nil {
		return nil, nil, err
	}

	//go through the config json to get to the procstat metrics
	procstatList := cfgFileData["metrics"].(map[string]interface{})["metrics_collected"].(map[string]interface{})["procstat"].([]interface{})
	
	//within procstat metrics, find cloudwatch-agent process
	cloudwatchIndex := -1
	for i, process := range procstatList {
		if process.(map[string]interface{})["exe"].(string) == "cloudwatch-agent" {
			cloudwatchIndex = i
		}
	}

	//check to see if the process was not found
	if  cloudwatchIndex == -1 {
		return nil, nil, errors.New("cloudwatch-agent process not found in cloudwatch agent config")
	}

	//use the index to get the rest of the path
	metricList := procstatList[cloudwatchIndex].(map[string]interface{})["measurement"].([]interface{})

	//convert the resulting []interface{} to []string and create matching metric ids for each one
	metricNames := make([]string, len(metricList))
	ids := make([]string, len(metricList))
	for i, metricName := range metricList {
		metricNames[i] = "procstat_" + metricName.(string)
		ids[i] = fmt.Sprint("m", i + 1)
	}

	return metricNames, ids, nil
}

// GenerateGetMetricInputStruct generates the struct required to make a query request to cloudwatch's GetMetrics
func GenerateGetMetricInputStruct(ids, metricNames []string, instanceId string, timeDiff int) (*cloudwatch.GetMetricDataInput, error) {
	if len(ids) != len(metricNames) {
		return nil, errors.New("Mismatching lengths of metric ids and metricNames")
	}
	
	if len(ids) == 0 || len(metricNames) == 0 || instanceId == "" || timeDiff == 0 {
		return nil, errors.New("Must supply metric ids, metric names, instance id, and time to collect metrics")
	}

	dimensionValue := instanceId
	metricDataQueries := []types.MetricDataQuery{}
	
	//generate list of individual metric requests
	for i, id := range ids {
		metricDataQueries = append(metricDataQueries, ConstructMetricDataQuery(id, Namespace, DimensionName, dimensionValue, metricNames[i], timeDiff))
	}
	
	timeNow := time.Now()
	input := &cloudwatch.GetMetricDataInput{
		EndTime:   aws.Time(time.Unix(timeNow.Unix(), 0)),
		StartTime: aws.Time(time.Unix(timeNow.Add(time.Duration(-timeDiff)*time.Minute).Unix(), 0)),
		MetricDataQueries: metricDataQueries,
	}

	return input, nil
}

// ConstructMetricDataQuery is a helper function for GenerateGetMetricInputStruct and constructs individual metric requests
func ConstructMetricDataQuery(id, namespace, dimensionName, dimensionValue, metricName string, timeDiff int) (types.MetricDataQuery) {
	query := types.MetricDataQuery{
		Id: aws.String(id),
		MetricStat: &types.MetricStat{
			Metric: &types.Metric{
				Namespace:  aws.String(namespace),
				MetricName: aws.String(metricName),
				Dimensions: []types.Dimension{
					types.Dimension{
						Name:  aws.String(dimensionName),
						Value: aws.String(dimensionValue),
					},
				},
			},
			Period: aws.Int32(int32(Period)),
			Stat:   aws.String(Stat),
		},
	}

	return query
}

func GetPerformanceMetrics(instanceId string, agentRuntime int, agentContext context.Context) ([]byte, error) {

	//load default configuration
	cfg, err := config.LoadDefaultConfig(agentContext)
	if err != nil {
		return nil, err
	}

	client := cloudwatch.NewFromConfig(cfg)

	//fetch names of metrics to request and generate corresponding ids
	metricNames, ids, err := GetConfigMetrics()
	if err != nil {
		return nil, err
	}

	//make input struct
	input, err := GenerateGetMetricInputStruct(ids, metricNames, instanceId, agentRuntime)
	if err != nil {
		return nil, err
	}

	//call to CloudWatch API
	metrics, err := client.GetMetricData(agentContext, input)
	if err != nil {
		return nil, err
	}

	//format data to json before passing output
	outputData, err := json.MarshalIndent(metrics.MetricDataResults, "", "  ")
	if err != nil {
		return nil, err
    }

	return outputData, nil
}