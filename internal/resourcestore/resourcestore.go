// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
	"context"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorCtx "github.com/aws/amazon-cloudwatch-agent/translator/context"
)

var (
	resourceStore *ResourceStore
	once          sync.Once
)

type ec2ProviderType func(string) ec2iface.EC2API

type ServiceNameProvider interface {
	ServiceName()
	startServiceProvider(metadataProvider ec2metadataprovider.MetadataProvider)
	getIAMRole(metadataProvider ec2metadataprovider.MetadataProvider)
	getEC2Tags(ec2API ec2iface.EC2API)
}

type eksInfo struct {
	ClusterName string
}

type ResourceStore struct {
	// mode should be EC2, ECS, EKS, and K8S
	mode string

	// ec2Info stores information about EC2 instances such as instance ID and
	// auto scaling groups
	ec2Info ec2Info

	// ekeInfo stores information about EKS such as cluster
	eksInfo eksInfo

	// serviceprovider stores information about possible service names
	// that we can attach to the resource ID
	serviceprovider serviceprovider

	// logFiles is a variable reserved for communication between OTEL components and LogAgent
	// in order to achieve process correlations where the key is the log file path and the value
	// is the service name
	// Example:
	// "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log": "cloudwatch-agent"
	logFiles map[string]string
}

func GetResourceStore() *ResourceStore {
	once.Do(func() {
		resourceStore = initResourceStore()
	})
	return resourceStore
}

func initResourceStore() *ResourceStore {
	// Get IMDS client and EC2 API client which requires region for authentication
	// These will be passed down to any object that requires access to IMDS or EC2
	// API client so we have single source of truth for credential
	rs := &ResourceStore{}
	metadataProvider := getMetaDataProvider()
	if translatorCtx.CurrentContext().Mode() != "" {
		rs.mode = translatorCtx.CurrentContext().Mode()
		log.Printf("I! resourcestore: ResourceStore mode is %s ", rs.mode)
	}
	switch rs.mode {
	case config.ModeEC2:
		rs.ec2Info = *newEC2Info(metadataProvider, getEC2Provider)
		go rs.ec2Info.initEc2Info()
	}
	serviceInfo := newServiceProvider(metadataProvider, getEC2Provider)
	go serviceInfo.startServiceProvider()
	rs.serviceprovider = *serviceInfo
	return rs
}

func (r *ResourceStore) Mode() string {
	return r.mode
}

func (r *ResourceStore) EC2Info() ec2Info {
	return r.ec2Info
}

func (r *ResourceStore) EKSInfo() eksInfo {
	return r.eksInfo
}

func (r *ResourceStore) LogFiles() map[string]string {
	return r.logFiles
}

func (r *ResourceStore) CreateLogFileRID(fileGlobPath string, filePath string) *cloudwatchlogs.Resource {
	return &cloudwatchlogs.Resource{
		AttributeMaps: []map[string]*string{
			{
				"PlatformType":         aws.String("AWS::EC2"),
				"EC2.InstanceId":       aws.String("i-123456789"),
				"EC2.AutoScalingGroup": aws.String("test-group"),
			},
		},
		KeyAttributes: &cloudwatchlogs.KeyAttributes{
			Name:        aws.String("myService"),
			Environment: aws.String("myEnvironment"),
		},
	}
}

func getMetaDataProvider() ec2metadataprovider.MetadataProvider {
	mdCredentialConfig := &configaws.CredentialConfig{}
	return ec2metadataprovider.NewMetadataProvider(mdCredentialConfig.Credentials(), retryer.GetDefaultRetryNumber())
}

func getEC2Provider(region string) ec2iface.EC2API {
	ec2CredentialConfig := &configaws.CredentialConfig{}
	ec2CredentialConfig.Region = region
	return ec2.New(
		ec2CredentialConfig.Credentials(),
		&aws.Config{
			LogLevel: configaws.SDKLogLevel(),
			Logger:   configaws.SDKLogger{},
		})
}

func getRegion(metadataProvider ec2metadataprovider.MetadataProvider) (string, error) {
	instanceDocument, err := metadataProvider.Get(context.Background())
	if err != nil {
		return "", err
	}
	return instanceDocument.Region, nil
}
