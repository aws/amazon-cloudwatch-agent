// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package compass

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-test/environment"
	"github.com/aws/amazon-cloudwatch-agent-test/util/awsservice"
	"github.com/aws/amazon-cloudwatch-agent-test/util/common"
)

const (
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	logLineId1       = "foo"
	logLineId2       = "bar"
	logFilePath      = "/tmp/cwagent_log_test.log"
	sleepForFlush    = 60 * time.Second
	retryWaitTime    = 30 * time.Second
	cwlPerfEndpoint  = "https://logs-perf.us-east-1.amazonaws.com"
	iadRegionalCode  = "us-east-1"

	entityType        = "@entity.KeyAttributes.Type"
	entityName        = "@entity.KeyAttributes.Name"
	entityEnvironment = "@entity.KeyAttributes.Environment"

	entityPlatform   = "@entity.Attributes.PlatformType"
	entityInstanceId = "@entity.Attributes.EC2.InstanceId"
)

var (
	logLineIds = []string{logLineId1, logLineId2}
	rnf        *cwlTypes.ResourceNotFoundException
	cwlClient  *cloudwatchlogs.Client
	ec2Client  *ec2.Client
)

type expectedEntity struct {
	entityType   string
	name         string
	environment  string
	platformType string
	instanceId   string
}

func init() {
	environment.RegisterEnvironmentMetaDataFlags()
	awsCfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(iadRegionalCode),
	)
	if err != nil {
		// handle error
		fmt.Println("There was an error trying to load default config: ", err)
		return
	}

	cwlClient = cloudwatchlogs.NewFromConfig(awsCfg, func(o *cloudwatchlogs.Options) {
		o.BaseEndpoint = aws.String(cwlPerfEndpoint)
	})
	ec2Client = ec2.NewFromConfig(awsCfg)

}

// TestWriteLogsToCloudWatch writes N number of logs, and then validates that the
// log events are associated with entities from CloudWatch Logs
func TestWriteLogsToCloudWatch(t *testing.T) {
	// this uses the {instance_id} placeholder in the agent configuration,
	// so we need to determine the host's instance ID for validation
	instanceId := awsservice.GetInstanceId()
	log.Printf("Found instance id %s", instanceId)

	defer awsservice.DeleteLogGroupAndStream(instanceId, instanceId)

	testCases := map[string]struct {
		agentConfigPath string
		iterations      int
		useEC2Tag       bool
		expectedEntity  expectedEntity
	}{
		"Compass/IAMRole": {
			agentConfigPath: filepath.Join("resources", "compass_default_log.json"),
			iterations:      1000,
			expectedEntity: expectedEntity{
				entityType:   "Service",
				name:         "cwa-e2e-iam-instance-profile",
				environment:  "ec2:default",
				platformType: "AWS::EC2",
				instanceId:   instanceId,
			},
		},
		"Compass/EC2Tags": {
			agentConfigPath: filepath.Join("resources", "compass_default_log.json"),
			iterations:      1000,
			useEC2Tag:       true,
			expectedEntity: expectedEntity{
				entityType:   "Service",
				name:         "compass-service-test",
				environment:  "ec2:default",
				platformType: "AWS::EC2",
				instanceId:   instanceId,
			},
		},
		"Compass/ServiceInConfig": {
			agentConfigPath: filepath.Join("resources", "compass_service_in_config.json"),
			iterations:      1000,
			expectedEntity: expectedEntity{
				entityType:   "Service",
				name:         "compass-service",
				environment:  "compass-environment",
				platformType: "AWS::EC2",
				instanceId:   instanceId,
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if testCase.useEC2Tag {
				input := &ec2.CreateTagsInput{
					Resources: []string{instanceId},
					Tags: []ec2Types.Tag{
						{
							Key:   aws.String("service"),
							Value: aws.String("compass-service-test"),
						},
					},
				}
				_, err := ec2Client.CreateTags(context.TODO(), input)
				assert.NoError(t, err)
			}
			id := uuid.New()
			f, err := os.Create(logFilePath + "-" + id.String())
			if err != nil {
				t.Fatalf("Error occurred creating log file for writing: %v", err)
			}
			common.DeleteFile(common.AgentLogFile)
			common.TouchFile(common.AgentLogFile)

			common.CopyFile(testCase.agentConfigPath, configOutputPath)

			common.StartAgent(configOutputPath, true, false)

			// ensure that there is enough time from the "start" time and the first log line,
			// so we don't miss it in the GetLogEvents call
			writeLogLines(t, f, testCase.iterations)
			time.Sleep(sleepForFlush)
			common.StopAgent()
			end := time.Now()

			// check CWL to ensure we got the expected entities in the log group
			ValidateEntity(t, instanceId, instanceId, &end, testCase.expectedEntity)

			f.Close()
			os.Remove(logFilePath + "-" + id.String())
		})
	}
}

func writeLogLines(t *testing.T, f *os.File, iterations int) {
	log.Printf("Writing %d lines to %s", iterations*len(logLineIds), f.Name())

	for i := 0; i < iterations; i++ {
		ts := time.Now()
		for _, id := range logLineIds {
			_, err := f.WriteString(fmt.Sprintf("%s - [%s] #%d This is a log line.\n", ts.Format(time.StampMilli), id, i))
			if err != nil {
				// don't need to fatal error here. if a log line doesn't get written, the count
				// when validating the log stream should be incorrect and fail there.
				t.Logf("Error occurred writing log line: %v", err)
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
}

// ValidateLogs queries a given LogGroup/LogStream combination given the start and end times, and executes an
// arbitrary validator function on the found logs.
func ValidateEntity(t *testing.T, logGroup, logStream string, end *time.Time, expectedEntity expectedEntity) {
	log.Printf("Checking log group/stream: %s/%s", logGroup, logStream)

	logGroupInfo, err := getLogGroup()
	for _, lg := range logGroupInfo {
		if *lg.LogGroupName == logGroup {
			log.Println("Log group " + *lg.LogGroupName + " exists")
			break
		}
	}
	assert.NoError(t, err)
	begin := end.Add(-sleepForFlush * 2)
	log.Printf("Start time is " + begin.String() + " and end time is " + end.String())
	queryId, err := getLogQueryId(logGroup, &begin, end)
	assert.NoError(t, err)
	log.Printf("queryId is " + *queryId)
	result, err := getQueryResult(queryId)
	assert.NoError(t, err)
	if !assert.NotZero(t, len(result)) {
		return
	}
	requiredEntityFields := map[string]bool{
		entityType:        false,
		entityName:        false,
		entityEnvironment: false,
		entityPlatform:    false,
		entityInstanceId:  false,
	}
	for _, field := range result[0] {
		switch aws.ToString(field.Field) {
		case entityType:
			requiredEntityFields[entityType] = true
			assert.Equal(t, expectedEntity.entityType, aws.ToString(field.Value))
		case entityName:
			requiredEntityFields[entityName] = true
			assert.Equal(t, expectedEntity.name, aws.ToString(field.Value))
		case entityEnvironment:
			requiredEntityFields[entityEnvironment] = true
			assert.Equal(t, expectedEntity.environment, aws.ToString(field.Value))
		case entityPlatform:
			requiredEntityFields[entityPlatform] = true
			assert.Equal(t, expectedEntity.platformType, aws.ToString(field.Value))
		case entityInstanceId:
			requiredEntityFields[entityInstanceId] = true
			assert.Equal(t, expectedEntity.instanceId, aws.ToString(field.Value))

		}
		fmt.Printf("%s: %s\n", aws.ToString(field.Field), aws.ToString(field.Value))
	}
	allEntityFieldsFound := true
	for _, value := range requiredEntityFields {
		if !value {
			allEntityFieldsFound = false
		}
	}
	assert.True(t, allEntityFieldsFound)
}

func getLogQueryId(logGroup string, since, until *time.Time) (*string, error) {
	var queryId *string
	params := &cloudwatchlogs.StartQueryInput{
		QueryString:  aws.String("fields @message, @entity.KeyAttributes.Type, @entity.KeyAttributes.Name, @entity.KeyAttributes.Environment, @entity.Attributes.PlatformType, @entity.Attributes.EC2.InstanceId"),
		LogGroupName: aws.String(logGroup),
	}
	if since != nil {
		params.StartTime = aws.Int64(since.UnixMilli())
	}
	if until != nil {
		params.EndTime = aws.Int64(until.UnixMilli())
	}
	attempts := 0

	for {
		output, err := cwlClient.StartQuery(context.Background(), params)
		attempts += 1

		if err != nil {
			if errors.As(err, &rnf) && attempts <= awsservice.StandardRetries {
				// The log group/stream hasn't been created yet, so wait and retry
				time.Sleep(retryWaitTime)
				continue
			}

			// if the error is not a ResourceNotFoundException, we should fail here.
			return queryId, err
		}
		queryId = output.QueryId
		return queryId, err
	}
}

func getQueryResult(queryId *string) ([][]cwlTypes.ResultField, error) {
	attempts := 0
	var results [][]cwlTypes.ResultField
	params := &cloudwatchlogs.GetQueryResultsInput{
		QueryId: aws.String(*queryId),
	}
	for {
		if attempts > awsservice.StandardRetries {
			return results, errors.New("exceeded retry count")
		}
		result, err := cwlClient.GetQueryResults(context.Background(), params)
		log.Printf("GetQueryResult status is: %v", result.Status)
		attempts += 1
		if result.Status != cwlTypes.QueryStatusComplete {
			log.Printf("GetQueryResult: sleeping for 5 seconds until status is complete")
			time.Sleep(5 * time.Second)
			continue
		}
		log.Printf("GetQueryResult: result length is %d", len(result.Results))
		if err != nil {
			if errors.As(err, &rnf) {
				// The log group/stream hasn't been created yet, so wait and retry
				time.Sleep(retryWaitTime)
				continue
			}

			// if the error is not a ResourceNotFoundException, we should fail here.
			return results, err
		}
		results = result.Results
		return results, err
	}
}

func getLogGroup() ([]cwlTypes.LogGroup, error) {
	attempts := 0
	var logGroups []cwlTypes.LogGroup
	params := &cloudwatchlogs.DescribeLogGroupsInput{}
	for {
		output, err := cwlClient.DescribeLogGroups(context.Background(), params)

		attempts += 1

		if err != nil {
			if errors.As(err, &rnf) && attempts <= awsservice.StandardRetries {
				// The log group/stream hasn't been created yet, so wait and retry
				time.Sleep(retryWaitTime)
				continue
			}

			// if the error is not a ResourceNotFoundException, we should fail here.
			return logGroups, err
		}
		logGroups = output.LogGroups
		return logGroups, err
	}
}
