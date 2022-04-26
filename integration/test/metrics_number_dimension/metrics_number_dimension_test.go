// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metrics_number_dimension

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

const configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
const configJSON = "/config.json"
const namespace = "MetricNumberDimensionTest"
const instanceId = "InstanceId"
const appendMetric = "append"

// @TODO use the value from plugins/outputs/cloudwatch/cloudwatch.go when https://github.com/aws/amazon-cloudwatch-agent/pull/361 is merged
const maxDimension = 30

//Let the agent run for 2 minutes. This will give agent enough time to call server
const agentRuntime = 2 * time.Minute

const targetString = "max MaxDimensions %v is less than than number of dimensions %v thus only taking the max number"

type input struct {
	resourcePath         string
	findTarget           bool
	numberDimensionsInCW int
	metricName           string
}

type metric struct {
	name  string
	value string
}

//Must run this test with parallel 1 since this will fail if more than one test is running at the same time
func TestNumberMetricDimension(t *testing.T) {

	parameters := []input{
		{
			resourcePath:         "resources/10_dimension",
			findTarget:           false,
			numberDimensionsInCW: 10,
			metricName:           "mem_used_percent",
		},
		// @TODO add when https://github.com/aws/amazon-cloudwatch-agent/pull/361 is merged
		// {resourcePath: "resources/30_dimension", findTarget: false, numberDimensionsInCW: 30, metricName: "mem_used_percent",},
		// {resourcePath: "resources/35_dimension", findTarget: true, numberDimensionsInCW: 30, metricName: "mem_used_percent",},
	}

	for _, parameter := range parameters {
		//before test run
		log.Printf("resource file location %s find target %t input number dimension %d metric name %s",
			parameter.resourcePath, parameter.findTarget, parameter.numberDimensionsInCW, parameter.metricName)

		target := fmt.Sprintf(targetString, maxDimension, parameter.numberDimensionsInCW)

		t.Run(fmt.Sprintf("resource file location %s find target %t", parameter.resourcePath, parameter.findTarget), func(t *testing.T) {
			test.CopyFile(parameter.resourcePath+configJSON, configOutputPath)
			test.StartAgent(configOutputPath)
			time.Sleep(agentRuntime)
			log.Printf("Agent has been running for : %s", agentRuntime.String())
			test.StopAgent()

			// test for target string
			output := test.ReadAgentOutput(agentRuntime)
			containsTarget := outputLogContainsTarget(output, target)
			if (parameter.findTarget && !containsTarget) || (!parameter.findTarget && containsTarget) {
				t.Errorf("Find target is %t contains target is %t", parameter.findTarget, containsTarget)
			}

			// test for cloud watch metrics
			cxt := context.Background()
			dimensionFilter := buildDimensionFilterList(parameter.numberDimensionsInCW)
			client := test.GetCWClient(cxt)
			listMetricsInput := cloudwatch.ListMetricsInput{
				MetricName: aws.String(parameter.metricName),
				Namespace:  aws.String(namespace),
				Dimensions: dimensionFilter,
			}
			data, err := client.ListMetrics(cxt, &listMetricsInput)
			if err != nil {
				t.Errorf("Error getting metric data %v", err)
			}
			if len(data.Metrics) == 0 {
				metrics := make([]metric, len(dimensionFilter))
				for i, filter := range dimensionFilter {
					metrics[i] = metric{
						name:  *filter.Name,
						value: *filter.Value,
					}
				}
				t.Errorf("No metrics found for dimension %v metric name %v namespace %v",
					metrics, parameter.metricName, namespace)
			}
		})
	}
}

func buildDimensionFilterList(appendDimension int) []types.DimensionFilter {
	// we append dimension from 0 to max number - 2
	// then we add dimension instance id
	// thus for max dimension 10, 0 to 8 + instance id = 10 dimension
	ec2InstanceId := test.GetInstanceId()
	dimensionFilter := make([]types.DimensionFilter, appendDimension)
	for i := 0; i < appendDimension-1; i++ {
		dimensionFilter[i] = types.DimensionFilter{
			Name:  aws.String(fmt.Sprintf("%s%d", appendMetric, i)),
			Value: aws.String(fmt.Sprintf("%s%d", appendMetric, i)),
		}
	}
	dimensionFilter[appendDimension-1] = types.DimensionFilter{
		Name:  aws.String(instanceId),
		Value: aws.String(ec2InstanceId),
	}
	return dimensionFilter
}

func outputLogContainsTarget(output string, targetString string) bool {
	log.Printf("Log file %s", output)
	contains := strings.Contains(output, targetString)
	log.Printf("Log file contains target string %t", contains)
	return contains
}
