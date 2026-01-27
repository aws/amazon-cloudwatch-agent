// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azuretagger

import "time"

const (
	// Metadata keys for Azure dimensions
	MdKeyInstanceID   = "InstanceId"
	MdKeyInstanceType = "InstanceType"
	MdKeyImageID      = "ImageId"

	// Azure-specific metadata keys
	MdKeyVMScaleSetName    = "VMScaleSetName"
	MdKeyResourceGroupName = "ResourceGroupName"
	MdKeySubscriptionID    = "SubscriptionId"

	// CloudWatch dimension for VMSS (Azure equivalent of ASG)
	CWDimensionVMSS = "VMScaleSetName"
)

var (
	// defaultRefreshInterval is the default interval for refreshing tags
	defaultRefreshInterval = 180 * time.Second

	// BackoffSleepArray defines retry intervals for initial tag retrieval
	BackoffSleepArray = []time.Duration{0, 1 * time.Minute, 1 * time.Minute, 3 * time.Minute, 3 * time.Minute, 3 * time.Minute, 10 * time.Minute}
)

// SupportedAppendDimensions maps dimension names to placeholder values for Azure
var SupportedAppendDimensions = map[string]string{
	"VMScaleSetName":    "${azure:VMScaleSetName}",
	"ImageId":           "${azure:ImageId}",
	"InstanceId":        "${azure:InstanceId}",
	"InstanceType":      "${azure:InstanceType}",
	"ResourceGroupName": "${azure:ResourceGroupName}",
	"SubscriptionId":    "${azure:SubscriptionId}",
}
