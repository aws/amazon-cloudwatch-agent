// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"strings"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

func GetECSClusterName(sectionKey string, input map[string]interface{}) string {
	var clusterName string
	if val, ok := input[sectionKey]; ok {
		//The key is in current input instance, use the value in JSON.
		clusterName = val.(string)
	}

	if clusterName == "" {
		clusterName = GetECSClusterNameFromEnv()
	}
	return clusterName
}

func GetECSClusterNameFromEnv() string {
	var clusterName string
	if ecsutil.GetECSUtilSingleton().IsECS() {
		clusterName = ecsutil.GetECSUtilSingleton().Cluster
		res := strings.Split(clusterName, "/")
		if len(res) > 0 {
			clusterName = res[len(res)-1]
		}
	}
	return clusterName
}