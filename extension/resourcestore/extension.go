// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

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

type ResourceStore struct {
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
	// that we can attach to the resource ID
	serviceprovider serviceProviderInterface

	// nativeCredential stores the credential config for agent's native
	// component such as LogAgent
	nativeCredential client.ConfigProvider

	metadataprovider ec2metadataprovider.MetadataProvider

	stsClient stsiface.STSAPI
}

var _ extension.Extension = (*ResourceStore)(nil)

func (r *ResourceStore) Start(ctx context.Context, host component.Host) error {
	// Get IMDS client and EC2 API client which requires region for authentication
	// These will be passed down to any object that requires access to IMDS or EC2
	// API client so we have single source of truth for credential
	r.done = make(chan struct{})
	r.metadataprovider = getMetaDataProvider()
	r.mode = r.config.Mode
	ec2CredentialConfig := &configaws.CredentialConfig{
		Profile:  r.config.Profile,
		Filename: r.config.Filename,
	}
	switch r.mode {
	case config.ModeEC2:
		r.ec2Info = *newEC2Info(r.metadataprovider, getEC2Provider, ec2CredentialConfig, r.done)
		go r.ec2Info.initEc2Info()
	}
	r.serviceprovider = newServiceProvider(r.mode, &r.ec2Info, r.metadataprovider, getEC2Provider, ec2CredentialConfig, r.done)
	go r.serviceprovider.startServiceProvider()
	return nil
}

func (r *ResourceStore) Shutdown(_ context.Context) error {
	close(r.done)
	return nil
}

func (r *ResourceStore) Mode() string {
	return r.mode
}

func (r *ResourceStore) EKSInfo() eksInfo {
	return r.eksInfo
}

func (r *ResourceStore) EC2Info() ec2Info {
	return r.ec2Info
}

func (r *ResourceStore) SetNativeCredential(client client.ConfigProvider) {
	r.nativeCredential = client
}

func (r *ResourceStore) NativeCredentialExists() bool {
	return r.nativeCredential != nil
}

// CreateLogFileRID creates the RID for log events that are being uploaded from a log file in the environment.
func (r *ResourceStore) CreateLogFileRID(logFileGlob LogFileGlob, logGroupName LogGroupName) *cloudwatchlogs.Resource {
	if !r.shouldReturnRID() {
		return nil
	}

	serviceAttr := r.serviceprovider.logFileServiceAttribute(logFileGlob, logGroupName)

	keyAttributes := r.createServiceKeyAttributes(serviceAttr)
	attributeMap := r.createAttributeMap()
	addNonEmptyToMap(attributeMap, ServiceNameSourceKey, serviceAttr.ServiceNameSource)

	return &cloudwatchlogs.Resource{
		KeyAttributes: keyAttributes,
		AttributeMaps: []map[string]*string{attributeMap},
	}
}

// AddServiceAttrEntryForLogFile adds an entry to the resource store for the provided file glob -> (serviceName, environmentName) key-value pair
func (r *ResourceStore) AddServiceAttrEntryForLogFile(fileGlob LogFileGlob, serviceName string, environmentName string) {
	if r.serviceprovider != nil {
		r.serviceprovider.addEntryForLogFile(fileGlob, ServiceAttribute{
			ServiceName:       serviceName,
			ServiceNameSource: ServiceNameSourceUserConfiguration,
			Environment:       environmentName,
		})
	}
}

// AddServiceAttrEntryForLogGroup adds an entry to the resource store for the provided log group nme -> (serviceName, environmentName) key-value pair
func (r *ResourceStore) AddServiceAttrEntryForLogGroup(logGroupName LogGroupName, serviceName string, environmentName string) {
	r.serviceprovider.addEntryForLogGroup(logGroupName, ServiceAttribute{
		ServiceName:       serviceName,
		ServiceNameSource: ServiceNameSourceInstrumentation,
		Environment:       environmentName,
	})
}

func (r *ResourceStore) createAttributeMap() map[string]*string {
	attributeMap := make(map[string]*string)

	if r.mode == config.ModeEC2 {
		addNonEmptyToMap(attributeMap, InstanceIDKey, r.ec2Info.InstanceID)
		addNonEmptyToMap(attributeMap, ASGKey, r.ec2Info.AutoScalingGroup)
	}
	switch r.mode {
	case config.ModeEC2:
		attributeMap[PlatformType] = aws.String(EC2PlatForm)
	}
	return attributeMap
}

// createServiceKeyAttribute creates KeyAttributes for Service resources
func (r *ResourceStore) createServiceKeyAttributes(serviceAttr ServiceAttribute) *cloudwatchlogs.KeyAttributes {
	serviceKeyAttr := &cloudwatchlogs.KeyAttributes{
		Type: aws.String(Service),
	}
	if serviceAttr.ServiceName != "" {
		serviceKeyAttr.SetName(serviceAttr.ServiceName)
	}
	if serviceAttr.Environment != "" {
		serviceKeyAttr.SetEnvironment(serviceAttr.Environment)
	}
	return serviceKeyAttr
}

// shouldReturnRID checks if the account ID for the instance is
// matching the account ID when assuming role for the current credential.
func (r *ResourceStore) shouldReturnRID() bool {
	if r.nativeCredential == nil || r.metadataprovider == nil {
		r.logger.Debug("there is no credential stored for cross-account checks")
		return false
	}
	doc, err := r.metadataprovider.Get(context.Background())
	if err != nil {
		r.logger.Debug("an error occurred when getting instance document for cross-account checks. Reason: %v\n", zap.Error(err))
		return false
	}
	instanceAccountID := doc.AccountID
	if r.stsClient == nil {
		r.stsClient = sts.New(
			r.nativeCredential,
			&aws.Config{
				LogLevel: configaws.SDKLogLevel(),
				Logger:   configaws.SDKLogger{},
			})
	}
	assumedRoleIdentity, err := r.stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		r.logger.Debug("an error occurred when calling STS GetCallerIdentity for cross-account checks. Reason: ", zap.Error(err))
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

func getRegion(metadataProvider ec2metadataprovider.MetadataProvider) (string, error) {
	instanceDocument, err := metadataProvider.Get(context.Background())
	if err != nil {
		return "", err
	}
	return instanceDocument.Region, nil
}

func addNonEmptyToMap(m map[string]*string, key, value string) {
	if value != "" {
		m[key] = aws.String(value)
	}
}
