// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stores

import (
	"github.com/docker/docker/pkg/testutil/assert"
	"strconv"
	"testing"
	"time"
)

func TestUtils_parseDeploymentFromReplicaSet(t *testing.T) {
	testcases := []struct {
		name         string
		inputString  string
		expected     string
	}{
		{
			name: "Get ReplicaSet Name with unallowed characters",
			inputString: "cloudwatch-agent",
			expected: "",
		},
		{
			name: "Get ReplicaSet Name with allowed characters",
			inputString: "cloudwatch-agent-42kcz",
			expected: "cloudwatch-agent",
		},
		{
			name: "Get ReplicaSet Name with string smaller than 3 characters",
			inputString: "cloudwatch-agent-sd",
			expected: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, parseDeploymentFromReplicaSet(tc.inputString))
		})
	}

	assert.Equal(t, "", parseDeploymentFromReplicaSet("cloudwatch-agent"))
	assert.Equal(t, "cloudwatch-agent", parseDeploymentFromReplicaSet("cloudwatch-agent-42kcz"))
}

func TestUtils_parseCronJobFromJob(t *testing.T) {
	testcases := []struct {
		name         string
		inputString  string
		expected     string
	}{
		{
			name: "Get CronJobControllerV2 Name after k8s v1.21 with correct Unix Time",
			inputString: "hello-"+strconv.FormatInt(time.Now().Unix()/60, 10),
			expected: "hello",
		},
		{
			name: "Get CronJobControllerV2 Name after k8s v1.21 with alphabet Unix Time",
			inputString: "hello-"+strconv.FormatInt(time.Now().Unix()/60, 10)+"abc",
			expected: "",
		},
		{
			name: "Get CronJobControllerV2 Name after k8s v1.21 with alphabet characters",
			inputString: "hello-"+strconv.FormatInt(time.Now().Unix()/60, 10)+"-name",
			expected: "",
		},
		{
			name: "Get CronJobControllerV2 Name after k8s v1.21 with Unix Time not equal to 10 letters",
			inputString: "hello"+strconv.FormatInt(time.Now().Unix()/60, 10)+"289",
			expected: "",
		},
		{
			name: "Get CronJob Name before k8s v1.21 with correct Unix Time",
			inputString: "hello-"+strconv.FormatInt(time.Now().Unix(), 10),
			expected: "hello",
		},
		{
			name: "Get CronJob Name before k8s v1.21 with alphabet characters",
			inputString: "hello-"+strconv.FormatInt(time.Now().Unix(), 10)+"-name",
			expected: "",
		},
		{
			name: "Get CronJob Name before k8s v1.21 with special characters",
			inputString: "hello-"+strconv.FormatInt(time.Now().Unix(), 10)+"&64",
			expected: "",
		},
		{
			name: "Get CronJob Name before k8s v1.21 with Unix Time not equal to 10 letters",
			inputString: "hello"+strconv.FormatInt(time.Now().Unix(), 10)+"-289",
			expected: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, parseCronJobFromJob(tc.inputString))
		})
	}
}
