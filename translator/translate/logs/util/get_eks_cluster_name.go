// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/tagutil"
)

// For ASG case, the ec2 tag may be not ready as soon as the node is started up.
// In this case, the translator will fail and then the pod will restart.
func GetEKSClusterName(sectionKey string, input map[string]interface{}) string {
	var clusterName string
	if val, ok := input[sectionKey]; ok {
		//The key is in current input instance, use the value in JSON.
		clusterName = val.(string)
	}
	if clusterName == "" {
		clusterName = GetClusterNameFromEc2Tagger()
	}
	return clusterName
}

func GetClusterNameFromEc2Tagger() string {
	instanceID := ec2util.GetEC2UtilSingleton().InstanceID
	if instanceID == "" {
		return ""
	}

	// Get all tags for the instance using the centralized tagutil
	allTags := tagutil.GetAllTagsForInstance(instanceID)

	// Look for kubernetes.io/cluster/<cluster-name> tags with value "owned"
	for tagKey, tagValue := range allTags {
		if strings.HasPrefix(tagKey, "kubernetes.io/cluster/") && tagValue == "owned" {
			clusterName := strings.TrimPrefix(tagKey, "kubernetes.io/cluster/")
			if clusterName != "" {
				return clusterName
			}
		}
	}

	// Fallback to custom EKS cluster name tag if exists
	if clusterName, exists := allTags["eks:cluster-name"]; exists {
		return clusterName
	}

	return ""
}
