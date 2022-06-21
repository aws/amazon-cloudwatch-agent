package data_collector

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
)
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

func GetPerformanceMetrics(instanceId string, runtimeSeconds int) ([]byte, error) {
	agentContext := context.TODO()

	//load default configuration
	cfg, err := config.LoadDefaultConfig(agentContext)
	if err != nil {
		return nil, err
	}

	client := cloudwatch.NewFromConfig(cfg)

	//get metric measurements from config file
	file, err := os.ReadFile("./resources/config.json")
	if err != nil {
		return nil, err
	}

	var cfgFileData map[string]interface{}
	err = json.Unmarshal(file, &cfgFileData)
	if err != nil {
		return nil, err
	}

	//go through the config json to get to the procstat metrics configured for cloudwatch agent
	procstatList := cfgFileData["metrics"].(map[string]interface{})["metrics_collected"].(map[string]interface{})["procstat"].([]interface{})
	
	//within procstat metrics, find cloudwatch-agent process in case more than one procstat process is configured
	cloudwatchIndex := -1
	for i, process := range procstatList {
		if process.(map[string]interface{})["exe"].(string) == "cloudwatch-agent" {
			cloudwatchIndex = i
		}
	}

	//check to see if the process was not found
	if  cloudwatchIndex == -1 {
		return nil, errors.New("cloudwatch-agent process not found in cloudwatch agent config")
	}

	//use the index to get the rest of the path
	metricList := procstatList[cloudwatchIndex].(map[string]interface{})["measurement"].([]interface{})

	//convert the resulting []interface{} to []string and create matching metric ids for each one
	numOfMetrics := len(metricList)
	metricNames := make([]string, numOfMetrics)
	ids := make([]string, numOfMetrics)
	for i, metricName := range metricList {
		metricNames[i] = "procstat_" + metricName.(string)
		ids[i] = fmt.Sprint("m", i + 1)
	}

	//make input struct
	input, err := GenerateGetMetricInputStruct(ids, metricNames, instanceId, runtimeSeconds)
	if err != nil {
		return nil, err
	}

	//call to cloudwatch agent API
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