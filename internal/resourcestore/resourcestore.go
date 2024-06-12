// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
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

type ServiceNameProvider interface {
	startServiceProvider(metadataProvider ec2metadataprovider.MetadataProvider)
	ServiceName()
	getIAMRole(metadataProvider ec2metadataprovider.MetadataProvider)
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
	// Add logic to store attributes such as instance ID, cluster name, etc here
	rs := &ResourceStore{}
	metadataProvider := getMetaDataProvider()
	if translatorCtx.CurrentContext().Mode() != "" {
		rs.mode = translatorCtx.CurrentContext().Mode()
		log.Printf("I! resourcestore: ResourceStore mode is %s ", rs.mode)
	}
	switch rs.mode {
	case config.ModeEC2:
		rs.ec2Info = ec2Info{
			metadataProvider: metadataProvider,
			credentialCfg:    &configaws.CredentialConfig{},
		}
		go rs.ec2Info.initEc2Info()
	}
	serviceInfo := newServiceProvider()
	go func() {
		err := serviceInfo.startServiceProvider(metadataProvider)
		if err != nil {
			log.Printf("E! resourcestore: Failed to start service provider: %v", err)
		}
	}()
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

func ec2Provider(ec2CredentialConfig *configaws.CredentialConfig) ec2iface.EC2API {
	return ec2.New(
		ec2CredentialConfig.Credentials(),
		&aws.Config{
			LogLevel: configaws.SDKLogLevel(),
			Logger:   configaws.SDKLogger{},
		})
}
