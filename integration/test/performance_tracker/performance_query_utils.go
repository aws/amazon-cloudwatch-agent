package data_collector

import (
	"time"
	"log"
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

const (
	Namespace = "CWAgent"
	DimensionName = "InstanceId"
	Stat = "Average"
	Period = 30
)

// CWGetMetricDataAPI defines the interface for the GetMetricData function
type CWGetMetricDataAPI interface {
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

// GetMetrics Fetches the cloudwatch metrics for your provided input in the given time-frame
func GetMetrics(c context.Context, api CWGetMetricDataAPI, input *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error) {
	return api.GetMetricData(c, input)
}

// GenerateGetMetricInputStruct generates the struct required to make a query request to cloudwatch's GetMetrics
func GenerateGetMetricInputStruct(ids []string, metricNames []string, instanceId string, timeDiff int) (*cloudwatch.GetMetricDataInput, error) {
	if len(ids) != len(metricNames) {
		log.Println("Error: Mismatching lengths of metric ids and metricNames")
		return nil, errors.New("Mismatching lengths of metric ids and metricNames")
	}
	
	if len(ids) == 0 || len(metricNames) == 0 || instanceId == "" || timeDiff == 0 {
		log.Println("Error: Must supply metric ids, metric names, instance id, and time to collect metrics")
	}

	dimensionValue := instanceId
	metricDataQueries := []types.MetricDataQuery{}
	
	//generate list of individual metric requests
	for i := 0; i < len(ids); i++ {
		metricDataQueries = append(metricDataQueries, ConstructMetricDataQuery(ids[i], Namespace, DimensionName, dimensionValue, metricNames[i], timeDiff))
	}
	
	input := &cloudwatch.GetMetricDataInput{
		EndTime:   aws.Time(time.Unix(time.Now().Unix(), 0)),
		StartTime: aws.Time(time.Unix(time.Now().Add(time.Duration(-timeDiff)*time.Minute).Unix(), 0)),
		MetricDataQueries: metricDataQueries,
	}

	return input, nil
}

// ConstructMetricDataQuery is a helper function for GenerateGetMetricInputStruct and constructs individual metric requests
func ConstructMetricDataQuery(id string, namespace string, dimensionName string, dimensionValue string, metricName string, timeDiff int) (types.MetricDataQuery) {
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
