// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

const (
	// InstanceId character maximum length is 19.
	// See https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_Instance.html.
	instanceIdSizeMax = 19

	// AutoScalingGroup character maximum length is 255.
	// See https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_AutoScalingGroup.html.
	autoScalingGroupSizeMax = 255
)

type EC2Info struct {
	InstanceID       string
	AccountID        string
	AutoScalingGroup string

	// region is used while making call to describeTags Ec2 API for AutoScalingGroup
	Region         string
	kubernetesMode string

	metadataProvider ec2metadataprovider.MetadataProvider
	logger           *zap.Logger
	done             chan struct{}
	mutex            sync.RWMutex
}

func (ei *EC2Info) initEc2Info() {
	if ei.metadataProvider == nil {
		return
	}
	ei.logger.Debug("Initializing EC2Info")
	if err := ei.setInstanceIDAccountID(); err != nil {
		return
	}
	// Instance metadata tags is not usable for EKS nodes
	// https://github.com/kubernetes/cloud-provider-aws/issues/762
	if ei.kubernetesMode != config.ModeEKS {
		limitedRetryer := NewRetryer(true, true, defaultJitterMin, defaultJitterMax, ec2tagger.BackoffSleepArray, maxRetry, ei.done, ei.logger)
		limitedRetryer.refreshLoop(ei.retrieveAsgName)
	}
	ei.logger.Debug("Finished initializing EC2Info")
}

func (ei *EC2Info) GetInstanceID() string {
	ei.mutex.RLock()
	defer ei.mutex.RUnlock()
	return ei.InstanceID
}

func (ei *EC2Info) GetAccountID() string {
	ei.mutex.RLock()
	defer ei.mutex.RUnlock()
	return ei.AccountID
}

func (ei *EC2Info) GetAutoScalingGroup() string {
	ei.mutex.RLock()
	defer ei.mutex.RUnlock()
	return ei.AutoScalingGroup
}

func (ei *EC2Info) setInstanceIDAccountID() error {
	for {
		metadataDoc, err := ei.metadataProvider.Get(context.Background())
		if err != nil {
			ei.logger.Debug("Failed to get Instance ID / Account ID through metadata provider", zap.Error(err))
			wait := time.NewTimer(1 * time.Minute)
			select {
			case <-ei.done:
				wait.Stop()
				return errors.New("shutdown signal received")
			case <-wait.C:
				continue
			}
		}
		ei.logger.Debug("Successfully retrieved Instance ID and Account ID")
		ei.mutex.Lock()
		ei.InstanceID = metadataDoc.InstanceID
		if idLength := len(ei.InstanceID); idLength > instanceIdSizeMax {
			ei.logger.Warn("InstanceId length exceeds characters limit and will be ignored", zap.Int("length", idLength), zap.Int("character limit", instanceIdSizeMax))
			ei.InstanceID = ""
		}
		ei.AccountID = metadataDoc.AccountID
		ei.mutex.Unlock()
		return nil
	}
}

func (ei *EC2Info) retrieveAsgName() error {
	tags, err := ei.metadataProvider.InstanceTags(context.Background())
	if err != nil {
		ei.logger.Debug("Failed to get tags through metadata provider", zap.Error(err))
		return err
	} else if strings.Contains(tags, ec2tagger.Ec2InstanceTagKeyASG) {
		asg, err := ei.metadataProvider.InstanceTagValue(context.Background(), ec2tagger.Ec2InstanceTagKeyASG)
		if err != nil {
			ei.logger.Error("Failed to get AutoScalingGroup through metadata provider", zap.Error(err))
			return err
		} else {
			ei.logger.Debug("AutoScalingGroup retrieved through IMDS")
			ei.mutex.Lock()
			ei.AutoScalingGroup = asg
			if asgLength := len(ei.AutoScalingGroup); asgLength > autoScalingGroupSizeMax {
				ei.logger.Warn("AutoScalingGroup length exceeds characters limit and will be ignored", zap.Int("length", asgLength), zap.Int("character limit", autoScalingGroupSizeMax))
				ei.AutoScalingGroup = ""
			}
			ei.mutex.Unlock()
		}
	}
	return nil
}

func newEC2Info(metadataProvider ec2metadataprovider.MetadataProvider, kubernetesMode string, done chan struct{}, region string, logger *zap.Logger) *EC2Info {
	return &EC2Info{
		metadataProvider: metadataProvider,
		kubernetesMode:   kubernetesMode,
		done:             done,
		Region:           region,
		logger:           logger,
	}
}
