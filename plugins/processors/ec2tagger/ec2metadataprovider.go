// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"

	configaws "github.com/aws/private-amazon-cloudwatch-agent-staging/cfg/aws"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/retryer"
)

type MetadataProvider interface {
	Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error)
	Hostname(ctx context.Context) (string, error)
	InstanceID(ctx context.Context) (string, error)
}

type metadataClient struct {
	metadataFallbackDisabled *ec2metadata.EC2Metadata
	metadataFallbackEnabled  *ec2metadata.EC2Metadata
}

var _ MetadataProvider = (*metadataClient)(nil)

func NewMetadataProvider(p client.ConfigProvider) MetadataProvider {
	disableFallbackConfig := &aws.Config{
		HTTPClient:                &http.Client{Timeout: defaultIMDSTimeout},
		LogLevel:                  configaws.SDKLogLevel(),
		Logger:                    configaws.SDKLogger{},
		Retryer:                   retryer.IMDSRetryer,
		EC2MetadataEnableFallback: aws.Bool(false),
	}
	enableFallbackConfig := &aws.Config{
		HTTPClient: &http.Client{Timeout: defaultIMDSTimeout},
		LogLevel:   configaws.SDKLogLevel(),
		Logger:     configaws.SDKLogger{},
	}
	return &metadataClient{
		metadataFallbackDisabled: ec2metadata.New(p, disableFallbackConfig),
		metadataFallbackEnabled:  ec2metadata.New(p, enableFallbackConfig),
	}
}

func (c *metadataClient) InstanceID(ctx context.Context) (string, error) {
	contextOuter, cancelFn := context.WithTimeout(ctx, 30*time.Second)
	defer cancelFn()
	instanceId, err := c.metadataFallbackDisabled.GetMetadataWithContext(contextOuter, "instance-id")
	if err != nil {
		log.Printf("D! could not get instance id without imds v1 fallback enable thus enable fallback")
		contextInner, cancelFnInner := context.WithTimeout(ctx, 30*time.Second)
		defer cancelFnInner()
		instanceIdInner, errInner := c.metadataFallbackEnabled.GetMetadataWithContext(contextInner, "instance-id")
		return instanceIdInner, errInner
	}
	return instanceId, err
}

func (c *metadataClient) Hostname(ctx context.Context) (string, error) {
	contextOuter, cancelFn := context.WithTimeout(ctx, 30*time.Second)
	defer cancelFn()
	hostname, err := c.metadataFallbackDisabled.GetMetadataWithContext(contextOuter, "hostname")
	if err != nil {
		log.Printf("D! could not get hostname without imds v1 fallback enable thus enable fallback")
		contextInner, cancelFnInner := context.WithTimeout(ctx, 30*time.Second)
		defer cancelFnInner()
		hostnameInner, errInner := c.metadataFallbackEnabled.GetMetadataWithContext(contextInner, "hostname")
		return hostnameInner, errInner
	}
	return hostname, err
}

func (c *metadataClient) Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	contextOuter, cancelFn := context.WithTimeout(ctx, 30*time.Second)
	defer cancelFn()
	instanceDocument, err := c.metadataFallbackDisabled.GetInstanceIdentityDocumentWithContext(contextOuter)
	if err != nil {
		log.Printf("D! could not get instance document without imds v1 fallback enable thus enable fallback")
		contextInner, cancelFnInner := context.WithTimeout(ctx, 30*time.Second)
		defer cancelFnInner()
		instanceDocumentInner, errInner := c.metadataFallbackEnabled.GetInstanceIdentityDocumentWithContext(contextInner)
		return instanceDocumentInner, errInner
	}
	return instanceDocument, err
}
