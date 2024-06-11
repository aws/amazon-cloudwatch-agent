// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
	"sync"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
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

type ec2Info struct {
	InstanceID       string
	AutoScalingGroup string
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
	metadataProvider := getMetaDataProvider()
	serviceInfo := newServiceProvider()
	go serviceInfo.startServiceProvider(metadataProvider)
	return &ResourceStore{
		serviceprovider: *serviceInfo,
	}
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

func getMetaDataProvider() ec2metadataprovider.MetadataProvider {
	mdCredentialConfig := &configaws.CredentialConfig{}
	return ec2metadataprovider.NewMetadataProvider(mdCredentialConfig.Credentials(), retryer.GetDefaultRetryNumber())
}
