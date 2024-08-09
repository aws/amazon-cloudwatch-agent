// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"time"
)

// Reminder, keep this in sync with the plugin's README.md
const sampleConfig = `
  ##
  ## ec2tagger calls AWS api to fetch EC2 Metadata and Instance Tags and EBS Volumes associated with the
  ## current EC2 Instance and attached those values as tags to the metric.
  ##
  ## Frequency for the plugin to refresh the EC2 Instance Tags and ebs Volumes associated with this Instance.
  ## Defaults to 0 (no refresh).
  ## When it is zero, ec2tagger doesn't do refresh to keep the ec2 tags and ebs volumes updated. However, as the
  ## AWS api request made by ec2tagger might not return the complete values (e.g. initial api call might return a
  ## subset of ec2 tags), ec2tagger will retry every 3 minutes until all the tags/volumes (as specified by
  ## "ec2_instance_tag_keys"/"ebs_device_keys") are retrieved successfully. (Note when the specified list is ["*"],
  ## there is no way to check if all tags/volumes are retrieved, so there is no retry in that case)
  # refresh_interval_seconds = 60
  ##
  ## Add tags for EC2 Metadata fields.
  ## Supported fields are: "InstanceId", "ImageId" (aka AMI), "InstanceType"
  ## If the configuration is not provided or it has an empty list, no EC2 Metadata tags are applied.
  # ec2_metadata_tags = ["InstanceId", "ImageId", "InstanceType"]
  ##
  ## Add tags retrieved from the EC2 Instance Tags associated with this instance.
  ## If this configuration is not provided, or has an empty list, no EC2 Instance Tags are applied.
  ## If this configuration contains one entry and its value is "*", then ALL EC2 Instance Tags for the instance are applied.
  ## Note: This plugin renames the "aws:autoscaling:groupName" EC2 Instance Tag key to be spelled "AutoScalingGroupName".
  ## This aligns it with the AutoScaling dimension-name seen in AWS CloudWatch.
  # ec2_instance_tag_keys = ["aws:autoscaling:groupName", "Name"]
  ##
  ## Retrieve ebs_volume_id for the specified devices, add ebs_volume_id as tag. The specified devices are
  ## the values corresponding to the tag key "disk_device_tag_key" in the input metric.
  ## If this configuration is not provided, or has an empty list, no ebs volume is applied.
  ## If this configuration contains one entry and its value is "*", then all ebs volume for the instance are applied.
  # ebs_device_keys = ["/dev/xvda", "/dev/nvme0n1"]
  ##
  ## Specify which tag to use to get the specified disk device name from input Metric
  # disk_device_tag_key = "device"
  ##
  ## Amazon Credentials
  ## Credentials are loaded in the following order
  ## 1) Assumed credentials via STS if role_arn is specified
  ## 2) explicit credentials from 'access_key' and 'secret_key'
  ## 3) shared profile from 'profile'
  ## 4) environment variables
  ## 5) shared credentials file
  ## 6) EC2 Instance Profile
  # access_key = ""
  # secret_key = ""
  # token = ""
  # role_arn = ""
  # profile = ""
  # shared_credential_file = ""
`

const (
	ec2InstanceTagKeyASG = "aws:autoscaling:groupName"
	cwDimensionASG       = "AutoScalingGroupName"
	mdKeyInstanceId      = "InstanceId"
	mdKeyImageId         = "ImageId"
	mdKeyInstanceType    = "InstanceType"
)

var (
	// issue with newer versions of the sdk take longer when hop limit is 1 in eks
	defaultRefreshInterval = 180 * time.Second
	backoffSleepArray      = []time.Duration{0, 1 * time.Minute, 1 * time.Minute, 3 * time.Minute, 3 * time.Minute, 3 * time.Minute, 10 * time.Minute} // backoff retry for ec2 describe instances API call. Assuming the throttle limit is 20 per second. 10 mins allow 12000 API calls.
)
