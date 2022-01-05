// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sdecorator

import (
	"log"
	"net/http"
	"strings"
	"time"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	SectionKeyClusterName   = "cluster_name"
	ClusterNameTagKeyPrefix = "kubernetes.io/cluster/"
	defaultRetryCount       = 5
)

var (
	sleeps = []time.Duration{time.Millisecond * 200, time.Millisecond * 400, time.Millisecond * 800, time.Millisecond * 1600, time.Millisecond * 3200}
)

type ClusterName struct {
}

func (c *ClusterName) ApplyRule(input interface{}) (string, interface{}) {
	clusterName := getClusterName(input.(map[string]interface{}))
	return SectionKeyClusterName, clusterName
}

// For ASG case, the ec2 tag may be not ready as soon as the node is started up.
// In this case, the translator will fail and then the pod will restart.
func getClusterName(kuberneteInput map[string]interface{}) string {
	var clusterName string
	if val, ok := kuberneteInput["cluster_name"]; ok {
		//The key is in current input instance, use the value in JSON.
		clusterName = val.(string)
	}
	if clusterName == "" {
		clusterName = getClusterNameFromEc2Tagger()
	}
	if clusterName == "" {
		translator.AddErrorMessages(GetCurPath(), "ClusterName is not defined")
	}
	return clusterName
}

func getClusterNameFromEc2Tagger() string {
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
		HTTPClient:                    &http.Client{Timeout: 1 * time.Minute},
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
			if strings.HasPrefix(key, ClusterNameTagKeyPrefix) && *tag.Value == "owned" {
				clusterName := key[len(ClusterNameTagKeyPrefix):]
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

func init() {
	RegisterRule(SectionKeyClusterName, new(ClusterName))
}

//encapsulate the retry logic in this separate method.
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

//sleep some back off time before retries.
func backoffSleep(i int) {
	//save the sleep time for the last occurrence since it will exit the loop immediately after the sleep
	backoffDuration := time.Duration(time.Minute * 1)
	if i <= defaultRetryCount {
		backoffDuration = sleeps[i]
	}

	log.Printf("W! It is the %v time, going to sleep %v before retrying.", i, backoffDuration)
	time.Sleep(backoffDuration)
}
