// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsutil

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/httpclient"
)

const (
	v2MetadataEndpoint    = "http://169.254.170.2/v2/metadata"
	v3MetadataEndpointEnv = "ECS_CONTAINER_METADATA_URI"
	v4MetadataEndpointEnv = "ECS_CONTAINER_METADATA_URI_V4"
)

type ecsMetadataResponse struct {
	Cluster string
	TaskARN string
}

type ecsUtil struct {
	Cluster    string
	Region     string
	TaskARN    string
	httpClient *httpclient.HttpClient
}

var ecsUtilOnce sync.Once

func GetECSUtilSingleton() (ecsUtilInstance *ecsUtil) {
	ecsUtilOnce.Do(func() {
		ecsUtilInstance = initECSUtilSingleton()
	})
	return ecsUtilInstance
}

func initECSUtilSingleton() (newInstance *ecsUtil) {
	newInstance = &ecsUtil{httpClient: httpclient.New()}
	if os.Getenv(config.RUN_IN_CONTAINER) != config.RUN_IN_CONTAINER_TRUE {
		return
	}
	log.Println("I! attempt to access ECS task metadata to determine whether I'm running in ECS.")
	ecsMetadataResponse, err := newInstance.getECSMetadata()

	if err != nil {
		log.Printf("I! access ECS task metadata fail with response %v, assuming I'm not running in ECS.\n", err)
		return
	}

	newInstance.parseRegion(ecsMetadataResponse)
	newInstance.Cluster = ecsMetadataResponse.Cluster
	newInstance.TaskARN = ecsMetadataResponse.TaskARN
	return

}

func (e *ecsUtil) IsECS() bool {
	return e.Region != ""
}

func (e *ecsUtil) getECSMetadata() (em *ecsMetadataResponse, err error) {
	// Based on endpoint to get ECS metadata, for more information on the respond, https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint.html
	if v4MetadataEndpoint, ok := os.LookupEnv(v4MetadataEndpointEnv).(string); ok {
		em, err = e.getMetadataResponse(v4MetadataEndpoint + "/task")
	} else if v3MetadataEndpoint, ok := os.LookupEnv(v3MetadataEndpointEnv).(string); ok {
		em, err = e.getMetadataResponse(v3MetadataEndpoint + "/task")
	} else {
		em, err = e.getMetadataResponse(v2MetadataEndpoint)
	}
	return
}

func (e *ecsUtil) getMetadataResponse(endpoint string) (em *ecsMetadataResponse, err error) {
	em = &ecsMetadataResponse{}
	resp, err := e.httpClient.Request(endpoint)
	
	if err != nil {
		return
	}

	err = json.Unmarshal(resp, em)
	if err != nil {
		log.Printf("E! Unable to parse response from ecsmetadata endpoint, error: %v", err)
		log.Printf("D! Respond content is %s", string(resp))
	}
	return
}

// There are two formats of Task ARN (https://docs.aws.amazon.com/AmazonECS/latest/userguide/ecs-account-settings.html#ecs-resource-ids)
// arn:aws:ecs:region:aws_account_id:task/task-id
// arn:aws:ecs:region:aws_account_id:task/cluster-name/task-id
// This function will return region extracted from Task ARN
func (e *ecsUtil) parseRegion(em *ecsMetadataResponse) {
	splitedContent := strings.Split(em.TaskARN, ":")
	// When splitting the ARN with ":", the 4th segment is the region
	if len(splitedContent) < 4 {
		log.Printf("E! invalid ecs task arn: %s", em.TaskARN)
	}
	e.Region = splitedContent[3]
}
