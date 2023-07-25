// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package defaultConfig

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/defaultConfig/advancedPlan"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/defaultConfig/basicPlan"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/defaultConfig/standardPlan"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration/linux"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/question"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/question/logs"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	if wantMonitorAnyHostMetrics() {
		wantPerInstanceMetrics(ctx)
		wantEC2TagDimensions(ctx)
		wantEC2AggregateDimensions(ctx)
		metricsCollectInterval(ctx)
	} else {
		if ctx.OsParameter == util.OsTypeWindows {
			return logs.Processor
		} else {
			return linux.Processor
		}
	}

	backupCtx, err := json.Marshal(ctx)
	if err != nil {
		fmt.Printf("Error occurred when marshal context object into json:\n %v\n", err)
	}
	backupConfig, err := json.Marshal(config)
	if err != nil {
		fmt.Printf("Error occurred when marshal config object into json:\n %v\n", err)
	}
	for {
		//This is to avoid golang import cycle not allowed issue, we need to go back to the parent if the user is not satisfied with the config.
		whichDefaultConfig := whichDefaultConfig()
		switch whichDefaultConfig {
		case "Basic":
			basicPlan.Processor.Process(ctx, config)
		case "Standard":
			standardPlan.Processor.Process(ctx, config)
		case "Advanced":
			advancedPlan.Processor.Process(ctx, config)
		case "None":
			return question.Processor
		default:
			log.Panicf("Unknown default config: %s", whichDefaultConfig)
		}
		if config.SatisfiedWithCurrentConfig(ctx) {
			if ctx.OsParameter == util.OsTypeWindows {
				return logs.Processor
			} else {
				return linux.Processor
			}
		} else {
			err := json.Unmarshal(backupCtx, ctx)
			if err != nil {
				fmt.Printf("Error occurred when unmarshal context object into json:\n %v\n", err)
			}
			err = json.Unmarshal(backupConfig, config)
			if err != nil {
				fmt.Printf("Error occurred when unmarshal config object into json:\n %v\n", err)
			}
		}
	}
}

func whichDefaultConfig() string {
	answer := util.Choice(
		"Which default metrics config do you want?",
		1,
		[]string{"Basic", "Standard", "Advanced", "None"})
	return answer
}

func wantMonitorAnyHostMetrics() bool {
	return util.Yes("Do you want to monitor any host metrics? e.g. CPU, memory, etc.")
}

func wantPerInstanceMetrics(ctx *runtime.Context) {
	ctx.WantPerInstanceMetrics = util.Yes("Do you want to monitor cpu metrics per core?")
}

func wantEC2TagDimensions(ctx *runtime.Context) {
	if ctx.IsOnPrem {
		return
	}
	ctx.WantEC2TagDimensions = util.Yes("Do you want to add ec2 dimensions (ImageId, InstanceId, InstanceType, AutoScalingGroupName) into all of your metrics if the info is available?")
}

func wantEC2AggregateDimensions(ctx *runtime.Context) {
	if ctx.IsOnPrem {
		return
	}
	ctx.WantAggregateDimensions = util.Yes("Do you want to aggregate ec2 dimensions (InstanceId)?")
}

func metricsCollectInterval(ctx *runtime.Context) {
	answer := util.Choice("Would you like to collect your metrics at high resolution (sub-minute resolution)? This enables sub-minute resolution for all metrics, but you can customize for specific metrics in the output json file.", 4, []string{"1s", "10s", "30s", "60s"})
	if val, err := strconv.Atoi(answer[:len(answer)-1]); err == nil {
		ctx.MetricsCollectionInterval = val
	} else {
		log.Panicf("Failed to parse the collect time interval. Error details: %v", err)
	}
}
