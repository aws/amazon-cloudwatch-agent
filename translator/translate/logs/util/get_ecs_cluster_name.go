// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

func GetECSClusterName(sectionKey string, input map[string]interface{}) string {
	var clusterName string
	if val, ok := input[sectionKey]; ok {
		//The key is in current input instance, use the value in JSON.
		clusterName = val.(string)
	}

	if clusterName == "" {
		clusterName = ecsutil.GetECSUtilSingleton().Cluster
	}
	return clusterName
}
