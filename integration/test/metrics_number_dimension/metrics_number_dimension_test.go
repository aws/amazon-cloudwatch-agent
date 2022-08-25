// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metrics_number_dimension

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/integration/test/util"
	cwPlugin "github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatch"
)

const (
	configJSON       = "/config.json"
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	namespace        = "MetricNumberDimensionTest"
	agentRuntime = 2 * time.Minute
)

type input struct {
	resourcePath         string
	failToStart          bool
	numberDimensionsInCW int
	metricName           string
}

// Must run this test with parallel 1 since this will fail if more than one test is running at the same time
func TestNumberMetricDimension(t *testing.T) {

	parameters := []input{
		{
			resourcePath:         "resources/30_dimension",
			failToStart:          false,
			numberDimensionsInCW: 30,
			metricName:           "mem_used_percent",
		},
		{
			resourcePath:         "resources/35_dimension",
			failToStart:          true,
			numberDimensionsInCW: 35,
			metricName:           "mem_used_percent",
		},
	}

	for _, parameter := range parameters {
		//before test run
		t.Logf("resource file location %s fail to start %t input number dimension %d metric name %s",
			parameter.resourcePath, parameter.failToStart, parameter.numberDimensionsInCW, parameter.metricName)

		t.Run(fmt.Sprintf("resource file location %s find target %t", parameter.resourcePath, parameter.failToStart), func(t *testing.T) {
			util.CopyFile(parameter.resourcePath+configJSON, configOutputPath)
			err := util.StartAgent(configOutputPath, false)

			// for append dimension we auto fail over 30 for custom metrics (statsd we collect remove dimension and continue)
			// Go output starts at the time of failure so the failure message gets chopped off. Thus have to use if there is an error and just assume reason.
			if parameter.failToStart && err == nil {
				t.Fatalf("Agent should not have started for append %v dimension", parameter.numberDimensionsInCW)
			} else if parameter.failToStart {
				t.Logf("Agent could not start due to appending more than %v dimension", cwPlugin.MaxDimensions)
				return
			}

			time.Sleep(agentRuntime)
			t.Logf("Agent has been running for : %s", agentRuntime.String())
			util.StopAgent()

			// test for cloud watch metrics
			dimensionFilter := util.BuildDimensionFilterList(parameter.numberDimensionsInCW)
			util.ValidateMetrics(t, parameter.metricName, namespace, dimensionFilter)

		})
	}
}
