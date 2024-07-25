// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

const (
	Service              = "Service"
	InstanceIDKey        = "EC2.InstanceId"
	ASGKey               = "EC2.AutoScalingGroup"
	ServiceNameSourceKey = "AWS.ServiceNameSource"
	PlatformType         = "PlatformType"
	EC2PlatForm          = "AWS::EC2"
	Type                 = "Type"
	Name                 = "Name"
	Environment          = "Environment"
)

type ec2ProviderType func(string, *configaws.CredentialConfig) ec2iface.EC2API

type serviceProviderInterface interface {
	startServiceProvider()
	addEntryForLogFile(LogFileGlob, ServiceAttribute)
	addEntryForLogGroup(LogGroupName, ServiceAttribute)
	logFileServiceAttribute(LogFileGlob, LogGroupName) ServiceAttribute
}

type eksInfo struct {
	ClusterName string
}

type EntityStore struct {
	logger *zap.Logger
	config *Config
	done   chan struct{}

	// mode should be EC2, ECS, EKS, and K8S
	mode string

	// ec2Info stores information about EC2 instances such as instance ID and
	// auto scaling groups
	ec2Info ec2Info

	// ekeInfo stores information about EKS such as cluster
	eksInfo eksInfo

	// serviceprovider stores information about possible service names
	// that we can attach to the entity
	serviceprovider serviceProviderInterface

	// nativeCredential stores the credential config for agent's native
	// component such as LogAgent
	nativeCredential client.ConfigProvider

	metadataprovider ec2metadataprovider.MetadataProvider

	stsClient stsiface.STSAPI
}

var _ extension.Extension = (*EntityStore)(nil)

func (e *EntityStore) Start(ctx context.Context, host component.Host) error {
	// Get IMDS client and EC2 API client which requires region for authentication
	// These will be passed down to any object that requires access to IMDS or EC2
	// API client so we have single source of truth for credential
	e.done = make(chan struct{})
	e.metadataprovider = getMetaDataProvider()
	e.mode = e.config.Mode
	ec2CredentialConfig := &configaws.CredentialConfig{
		Profile:  e.config.Profile,
		Filename: e.config.Filename,
	}
	switch e.mode {
	case config.ModeEC2:
		e.ec2Info = *newEC2Info(e.metadataprovider, getEC2Provider, ec2CredentialConfig, e.done, e.config.Region)
		go e.ec2Info.initEc2Info()
	}
	e.serviceprovider = newServiceProvider(e.mode, e.config.Region, &e.ec2Info, e.metadataprovider, getEC2Provider, ec2CredentialConfig, e.done)
	go e.serviceprovider.startServiceProvider()
	return nil
}

func (e *EntityStore) Shutdown(_ context.Context) error {
	close(e.done)
	return nil
}

func (e *EntityStore) Mode() string {
	return e.mode
}

func (e *EntityStore) EKSInfo() eksInfo {
	return e.eksInfo
}

func (e *EntityStore) EC2Info() ec2Info {
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
	if !e.shouldReturnEntity() {
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
	e.serviceprovider.addEntryForLogGroup(logGroupName, ServiceAttribute{
		ServiceName:       serviceName,
		ServiceNameSource: ServiceNameSourceInstrumentation,
		Environment:       environmentName,
	})
}

func (e *EntityStore) createAttributeMap() map[string]*string {
	attributeMap := make(map[string]*string)

	if e.mode == config.ModeEC2 {
		addNonEmptyToMap(attributeMap, InstanceIDKey, e.ec2Info.InstanceID)
		addNonEmptyToMap(attributeMap, ASGKey, e.ec2Info.AutoScalingGroup)
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
		Type: aws.String(Service),
	}
	addNonEmptyToMap(serviceKeyAttr, Name, serviceAttr.ServiceName)
	addNonEmptyToMap(serviceKeyAttr, Environment, serviceAttr.Environment)
	return serviceKeyAttr
}

// shouldReturnEntity checks if the account ID for the instance is
// matching the account ID when assuming role for the current credential.
func (e *EntityStore) shouldReturnEntity() bool {
	if e.nativeCredential == nil || e.metadataprovider == nil {
		e.logger.Debug("there is no credential stored for cross-account checks")
		return false
	}
	doc, err := e.metadataprovider.Get(context.Background())
	if err != nil {
		e.logger.Debug("an error occurred when getting instance document for cross-account checks. Reason: %v\n", zap.Error(err))
		return false
	}
	instanceAccountID := doc.AccountID
	if e.stsClient == nil {
		e.stsClient = sts.New(
			e.nativeCredential,
			&aws.Config{
				LogLevel: configaws.SDKLogLevel(),
				Logger:   configaws.SDKLogger{},
			})
	}
	assumedRoleIdentity, err := e.stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		e.logger.Debug("an error occurred when calling STS GetCallerIdentity for cross-account checks. Reason: ", zap.Error(err))
		return false
	}
	return instanceAccountID == *assumedRoleIdentity.Account
}

func getMetaDataProvider() ec2metadataprovider.MetadataProvider {
	mdCredentialConfig := &configaws.CredentialConfig{}
	return ec2metadataprovider.NewMetadataProvider(mdCredentialConfig.Credentials(), retryer.GetDefaultRetryNumber())
}

func getEC2Provider(region string, ec2CredentialConfig *configaws.CredentialConfig) ec2iface.EC2API {
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
