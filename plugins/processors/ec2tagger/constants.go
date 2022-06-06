// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"time"
)

const (
	ec2InstanceTagKeyASG = "aws:autoscaling:groupName"
	cwDimensionASG       = "AutoScalingGroupName"
	mdKeyInstanceId      = "InstanceId"
	mdKeyImageId         = "ImageId"
	mdKeyInstaneType     = "InstanceType"
	ebsVolumeId          = "EBSVolumeId"
)

const (
	metadataCheckStrStartInitialization         = "ec2tagger: EC2 IMDS has started initialization."
	metadataCheckStrTagNotSupported             = "ec2tagger: Unsupported EC2 Metadata key: %s"
	metadataCheckStrInstanceDocumentFailure     = "ec2tagger: Unable to retrieve Instance Metadata Tags: %+v."
	metadataCheckStrEC2InstanceTagger           = "ec2tagger: This plugin must only be used on an EC2 instance"
	metadataCheckStrIncreaseHopLimit            = "ec2tagger: Please increase hop limit to 3. For more instructions, please follow https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-options.html#configuring-IMDS-existing-instances."
	ec2TagAndVolumeCheckStrInitRetrievalSuccess = "ec2tagger: Initial retrieval of tags succeeded"
	ec2TagAndVolumeCheckStrStartInitialization  = "ec2tagger: EC2 tagger has started initialization."
	ec2VolumeCheckStrInitRetrievalFailure       = "ec2tagger: Unable to describe ec2 volume for initial retrieval: %v"
	ec2TagCheckStrInitRetrievalFailure          = "ec2tagger: Unable to describe ec2 tags for initial retrieval: %v"
	ec2TagAndVolumeCheckStrStartRefresh         = "ec2tagger refreshing: EC2InstanceTags needed %v, retrieved: %v, ebs device needed %v, retrieved: %v"
	ec2TagAndVolumeCheckStrStopRefresh          = "ec2tagger: Refresh is no longer needed, stop refreshTicker."
	ec2TagAndVolumeCheckStrRetryFailure         = "ec2tagger: %v retry initial retrieval of tags and volumes"
	ec2VolumeCheckStrRefreshFailure             = "ec2tagger: Error refreshing EC2 volumes, keeping old values : %+v"
	ec2TagCheckStrRefreshFailure                = "ec2tagger: Error refreshing EC2 tags, keeping old values : %+v"
	ec2TagAndVolumeCheckStrSuccess              = "ec2tagger: Finished initial retrieval of tags and volumes"
)

var (
	defaultRefreshInterval = 180 * time.Second
	// backoff retry for ec2 describe instances API call. Assuming the throttle limit is 20 per second. 10 mins allow 12000 API calls.
	backoffSleepArray = []time.Duration{0, 1 * time.Minute, 1 * time.Minute, 3 * time.Minute, 3 * time.Minute, 3 * time.Minute, 10 * time.Minute}
)