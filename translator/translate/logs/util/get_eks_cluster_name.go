// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
)

const (
	EKSClusterNameTagKeyPrefix = "kubernetes.io/cluster/"
	defaultRetryCount          = 5
)

var (
	sleeps = []time.Duration{time.Millisecond * 200, time.Millisecond * 400, time.Millisecond * 800, time.Millisecond * 1600, time.Millisecond * 3200}
)

// For ASG case, the ec2 tag may be not ready as soon as the node is started up.
// In this case, the translator will fail and then the pod will restart.
func GetEKSClusterName(sectionKey string, input map[string]interface{}) string {
	var clusterName string
	if val, ok := input[sectionKey]; ok {
		//The key is in current input instance, use the value in JSON.
		clusterName = val.(string)
	}
	if clusterName == "" {
		clusterName = GetClusterNameFromEc2Tagger()
	}
	return clusterName
}

func GetClusterNameFromEc2Tagger() string {
	instanceId := ec2util.GetEC2UtilSingleton().InstanceID
	region := ec2util.GetEC2UtilSingleton().Region

	if instanceId == "" || region == "" {
		return ""
	}

	tagFilters := []*ec2.Filter{
		{
			Name:   aws.String("resource-type"),
			Values: aws.StringSlice([]string{"instance"}),
		},
		{
			Name:   aws.String("resource-id"),
			Values: aws.StringSlice([]string{instanceId}),
		},
	}

	config := &aws.Config{
		Region:                        aws.String(region),
		CredentialsChainVerboseErrors: aws.Bool(true),
		LogLevel:                      configaws.SDKLogLevel(),
		Logger:                        configaws.SDKLogger{},
	}

	input := &ec2.DescribeTagsInput{
		Filters: tagFilters,
	}

	ses, err := session.NewSession(config)
	if err != nil {
		log.Println("E! getting new session info: ", err)
		return ""
	}
	ec2 := ec2.New(ses)
	for {
		result, err := callFuncWithRetries(ec2.DescribeTags, input, "Describe EC2 Tag Fail.")
		if err != nil {
			log.Println("E! DescribeTags EC2 tagger failed: ", err)
			return ""
		}
		for _, tag := range result.Tags {
			key := *tag.Key
			if strings.HasPrefix(key, EKSClusterNameTagKeyPrefix) && *tag.Value == "owned" {
				clusterName := key[len(EKSClusterNameTagKeyPrefix):]
				return clusterName
			}
		}
		if nil == result.NextToken {
			break
		}
		input.SetNextToken(*result.NextToken)
	}
	return ""
}

// encapsulate the retry logic in this separate method.
func callFuncWithRetries(fn func(input *ec2.DescribeTagsInput) (*ec2.DescribeTagsOutput, error), input *ec2.DescribeTagsInput, errorMsg string) (result *ec2.DescribeTagsOutput, err error) {
	for i := 0; i <= defaultRetryCount; i++ {
		result, err = fn(input)
		if err == nil {
			return result, nil
		}
		log.Printf("%s Will retry the request: %s", errorMsg, err.Error())
		backoffSleep(i)
	}
	return
}

// sleep some back off time before retries.
func backoffSleep(i int) {
	//save the sleep time for the last occurrence since it will exit the loop immediately after the sleep
	backoffDuration := time.Duration(time.Minute * 1)
	if i <= defaultRetryCount {
		backoffDuration = sleeps[i]
	}

	log.Printf("W! It is the %v time, going to sleep %v before retrying.", i, backoffDuration)
	time.Sleep(backoffDuration)
}
