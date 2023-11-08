// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-test/environment"
	"github.com/aws/amazon-cloudwatch-agent-test/util/awsservice"
	"github.com/aws/amazon-cloudwatch-agent-test/util/common"
)

const (
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	logFilePath      = "/tmp/test.log"
	agentRuntime     = 20 * time.Second // default flush interval is 5 seconds
)

var (
	ctx        = context.Background()
	awsCfg, _  = config.LoadDefaultConfig(ctx)
	CwlClient  = cloudwatchlogs.NewFromConfig(awsCfg)
	instanceId = awsservice.GetInstanceId()
)

type input struct {
	testName      string
	configPath    string
	logGroupName  string
	logGroupClass types.LogGroupClass
}

var testParameters = []input{
	{
		testName:      "Standard log config",
		configPath:    "testdata/logs_config.json",
		logGroupName:  instanceId,
		logGroupClass: types.LogGroupClassStandard,
	},
	{
		testName:      "Standard log config with standard class specification",
		configPath:    "testdata/logs_config_standard.json",
		logGroupName:  instanceId,
		logGroupClass: types.LogGroupClassStandard,
	},
	{
		testName:      "Standard log config with Infrequent_access class specification",
		configPath:    "testdata/logs_config_infrequent_access.json",
		logGroupName:  instanceId + "-infrequent-access",
		logGroupClass: types.LogGroupClassInfrequentAccess,
	},
}

func init() {
	environment.RegisterEnvironmentMetaDataFlags()
}

func TestWriteLogsToCloudWatch(t *testing.T) {
	logFile, err := os.Create(logFilePath)
	if err != nil {
		t.Fatalf("Error occurred creating log file for writing: %v", err)
	}
	defer logFile.Close()
	defer os.Remove(logFilePath)

	for run, param := range testParameters {
		t.Run(param.testName, func(t *testing.T) {
			defer awsservice.DeleteLogGroupAndStream(param.logGroupName, instanceId)
			common.DeleteFile(common.AgentLogFile)
			common.TouchFile(common.AgentLogFile)

			common.CopyFile(param.configPath, configOutputPath)

			err := common.StartAgent(configOutputPath, true, false)
			assert.Nil(t, err)
			// ensure that there is enough time from the "start" time and the first log line,
			time.Sleep(agentRuntime)
			_, err = logFile.WriteString(fmt.Sprintf("%s - [%s] #%d This is a log line.\n", time.Now().Format(time.StampMilli), "test", run))
			assert.Nil(t, err, "Error occurred writing log line: %v", err)
			time.Sleep(agentRuntime)
			common.StopAgent()

			agentLog, err := os.ReadFile(common.AgentLogFile)
			if err != nil {
				return
			}
			t.Logf("Agent logs %s", string(agentLog))

			valid, err := validateLogGroupExistence(param)
			assert.NoError(t, err)
			assert.True(t, valid)
		})
	}
}

func validateLogGroupExistence(param input) (bool, error) {
	// check CWL to ensure we got the expected log groups
	describeLogGroupInput := cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(param.logGroupName),
		LogGroupClass:      param.logGroupClass,
	}

	describeLogGroupOutput, err := CwlClient.DescribeLogGroups(ctx, &describeLogGroupInput)

	if err != nil {
		log.Println("error occurred while calling DescribeLogGroups", err)
		return false, err
	}

	return len(describeLogGroupOutput.LogGroups) > 0, nil
}
