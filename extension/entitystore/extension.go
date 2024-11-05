// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

const (
	Service                     = "Service"
	InstanceIDKey               = "EC2.InstanceId"
	ASGKey                      = "EC2.AutoScalingGroup"
	ServiceNameSourceKey        = "AWS.ServiceNameSource"
	PlatformType                = "PlatformType"
	EC2PlatForm                 = "AWS::EC2"
	podTerminationCheckInterval = 5 * time.Minute
)

type ec2ProviderType func(string, *configaws.CredentialConfig) ec2iface.EC2API

type serviceProviderInterface interface {
	startServiceProvider()
	addEntryForLogFile(LogFileGlob, ServiceAttribute)
	addEntryForLogGroup(LogGroupName, ServiceAttribute)
	logFileServiceAttribute(LogFileGlob, LogGroupName) ServiceAttribute
	getServiceNameAndSource() (string, string)
}

type EntityStore struct {
	logger *zap.Logger
	config *Config
	done   chan struct{}
	ready  bool

	// mode should be EC2, ECS, EKS, and K8S
	mode string

	kubernetesMode string

	// ec2Info stores information about EC2 instances such as instance ID and
	// auto scaling groups
	ec2Info EC2Info

	// eksInfo stores information about EKS such as pod to service Env map
	eksInfo *eksInfo

	// serviceprovider stores information about possible service names
	// that we can attach to the entity
	serviceprovider serviceProviderInterface

	// nativeCredential stores the credential config for agent's native
	// component such as LogAgent
	nativeCredential client.ConfigProvider

	metadataprovider ec2metadataprovider.MetadataProvider

	podTerminationCheckInterval time.Duration
}

var _ extension.Extension = (*EntityStore)(nil)

func (e *EntityStore) Start(ctx context.Context, host component.Host) error {
	// Get IMDS client and EC2 API client which requires region for authentication
	// These will be passed down to any object that requires access to IMDS or EC2
	// API client so we have single source of truth for credential
	e.done = make(chan struct{})
	e.metadataprovider = getMetaDataProvider()
	e.mode = e.config.Mode
	e.kubernetesMode = e.config.KubernetesMode
	e.podTerminationCheckInterval = podTerminationCheckInterval
	ec2CredentialConfig := &configaws.CredentialConfig{
		Profile:  e.config.Profile,
		Filename: e.config.Filename,
	}
	e.serviceprovider = newServiceProvider(e.mode, e.config.Region, &e.ec2Info, e.metadataprovider, getEC2Provider, ec2CredentialConfig, e.done, e.logger)
	switch e.mode {
	case config.ModeEC2:
		e.ec2Info = *newEC2Info(e.metadataprovider, e.done, e.config.Region, e.logger)
		go e.ec2Info.initEc2Info()
		go e.serviceprovider.startServiceProvider()
	}
	if e.kubernetesMode != "" {
		e.eksInfo = newEKSInfo(e.logger)
		// Starting the ttl cache will automatically evict all expired pods from the map
		go e.StartPodToServiceEnvironmentMappingTtlCache()
	}
	e.ready = true
	return nil
}

func (e *EntityStore) Shutdown(_ context.Context) error {
	close(e.done)
	if e.eksInfo != nil && e.eksInfo.podToServiceEnvMap != nil {
		e.eksInfo.podToServiceEnvMap.Stop()
	}
	e.logger.Info("Pod to Service Environment Mapping TTL Cache stopped")
	return nil
}

func (e *EntityStore) Mode() string {
	return e.mode
}

func (e *EntityStore) KubernetesMode() string {
	return e.kubernetesMode
}

func (e *EntityStore) EKSInfo() *eksInfo {
	return e.eksInfo
}

func (e *EntityStore) EC2Info() EC2Info {
	return e.ec2Info
}

func (e *EntityStore) SetNativeCredential(client client.ConfigProvider) {
	e.nativeCredential = client
}

func (e *EntityStore) NativeCredentialExists() bool {
	return e.nativeCredential != nil
}

// CreateLogFileEntity creates the entity for log events that are being uploaded from a log file in the environment.
func (e *EntityStore) CreateLogFileEntity(logFileGlob LogFileGlob, logGroupName LogGroupName) *cloudwatchlogs.Entity {
	if e.serviceprovider == nil {
		return nil
	}
	serviceAttr := e.serviceprovider.logFileServiceAttribute(logFileGlob, logGroupName)

	keyAttributes := e.createServiceKeyAttributes(serviceAttr)
	attributeMap := e.createAttributeMap()
	addNonEmptyToMap(attributeMap, ServiceNameSourceKey, serviceAttr.ServiceNameSource)

	return &cloudwatchlogs.Entity{
		KeyAttributes: keyAttributes,
		Attributes:    attributeMap,
	}
}

// GetMetricServiceNameAndSource gets the service name source for service metrics if not customer provided
func (e *EntityStore) GetMetricServiceNameAndSource() (string, string) {
	if e.serviceprovider == nil {
		return "", ""
	}
	return e.serviceprovider.getServiceNameAndSource()
}

// GetServiceMetricAttributesMap creates the attribute map for service metrics. This will be expanded upon in a later PR'S,
// but for now is just covering the EC2 attributes for service metrics.
func (e *EntityStore) GetServiceMetricAttributesMap() map[string]*string {
	return e.createAttributeMap()
}

// AddServiceAttrEntryForLogFile adds an entry to the entity store for the provided file glob -> (serviceName, environmentName) key-value pair
func (e *EntityStore) AddServiceAttrEntryForLogFile(fileGlob LogFileGlob, serviceName string, environmentName string) {
	if e.serviceprovider != nil {
		e.serviceprovider.addEntryForLogFile(fileGlob, ServiceAttribute{
			ServiceName:       serviceName,
			ServiceNameSource: ServiceNameSourceUserConfiguration,
			Environment:       environmentName,
		})
	}
}

// AddServiceAttrEntryForLogGroup adds an entry to the entity store for the provided log group nme -> (serviceName, environmentName) key-value pair
func (e *EntityStore) AddServiceAttrEntryForLogGroup(logGroupName LogGroupName, serviceName string, environmentName string) {
	if e.serviceprovider != nil {
		e.serviceprovider.addEntryForLogGroup(logGroupName, ServiceAttribute{
			ServiceName:       serviceName,
			ServiceNameSource: ServiceNameSourceInstrumentation,
			Environment:       environmentName,
		})
	}
}

func (e *EntityStore) AddPodServiceEnvironmentMapping(podName string, serviceName string, environmentName string, serviceNameSource string) {
	if e.eksInfo != nil {
		e.eksInfo.AddPodServiceEnvironmentMapping(podName, serviceName, environmentName, serviceNameSource)
	}
}

func (e *EntityStore) StartPodToServiceEnvironmentMappingTtlCache() {
	if e.eksInfo != nil && e.eksInfo.GetPodServiceEnvironmentMapping() != nil {
		e.eksInfo.GetPodServiceEnvironmentMapping().Start()
	}
}

func (e *EntityStore) GetPodServiceEnvironmentMapping() *ttlcache.Cache[string, ServiceEnvironment] {
	if e.eksInfo != nil {
		return e.eksInfo.GetPodServiceEnvironmentMapping()
	}
	return ttlcache.New[string, ServiceEnvironment](
		ttlcache.WithTTL[string, ServiceEnvironment](ttlDuration),
	)
}

func (e *EntityStore) createAttributeMap() map[string]*string {
	attributeMap := make(map[string]*string)

	if e.mode == config.ModeEC2 {
		addNonEmptyToMap(attributeMap, InstanceIDKey, e.ec2Info.GetInstanceID())
		addNonEmptyToMap(attributeMap, ASGKey, e.ec2Info.GetAutoScalingGroup())
	}
	switch e.mode {
	case config.ModeEC2:
		attributeMap[PlatformType] = aws.String(EC2PlatForm)
	}
	return attributeMap
}

// createServiceKeyAttribute creates KeyAttributes for Service entities
func (e *EntityStore) createServiceKeyAttributes(serviceAttr ServiceAttribute) map[string]*string {
	serviceKeyAttr := map[string]*string{
		entityattributes.EntityType: aws.String(Service),
	}
	addNonEmptyToMap(serviceKeyAttr, entityattributes.ServiceName, serviceAttr.ServiceName)
	addNonEmptyToMap(serviceKeyAttr, entityattributes.DeploymentEnvironment, serviceAttr.Environment)
	return serviceKeyAttr
}

var getMetaDataProvider = func() ec2metadataprovider.MetadataProvider {
	mdCredentialConfig := &configaws.CredentialConfig{}
	return ec2metadataprovider.NewMetadataProvider(mdCredentialConfig.Credentials(), retryer.GetDefaultRetryNumber())
}

var getEC2Provider = func(region string, ec2CredentialConfig *configaws.CredentialConfig) ec2iface.EC2API {
	ec2CredentialConfig.Region = region
	return ec2.New(
		ec2CredentialConfig.Credentials(),
		&aws.Config{
			LogLevel: configaws.SDKLogLevel(),
			Logger:   configaws.SDKLogger{},
		})
}

func addNonEmptyToMap(m map[string]*string, key, value string) {
	if value != "" {
		m[key] = aws.String(value)
	}
}
