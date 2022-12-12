// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stores

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUtils_parseDeploymentFromReplicaSet(t *testing.T) {
	testcases := []struct {
		name        string
		inputString string
		expected    string
	}{
		{
			name:        "Get ReplicaSet Name with unallowed characters",
			inputString: "cloudwatch-ag",
			expected:    "",
		},
		{
			name:        "Get ReplicaSet Name with allowed characters smaller than 3 characters",
			inputString: "cloudwatch-agent-bj",
			expected:    "",
		},
		{
			name:        "Get ReplicaSet Name with allowed characters",
			inputString: "cloudwatch-agent-42kcz",
			expected:    "cloudwatch-agent",
		},
		{
			name:        "Get ReplicaSet Name with string smaller than 3 characters",
			inputString: "cloudwatch-agent-sd",
			expected:    "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, parseDeploymentFromReplicaSet(tc.inputString), tc.expected)
		})
	}
}

func TestUtils_parseCronJobFromJob(t *testing.T) {
	unixTime := time.Now().Unix()
	unixTimeString := strconv.FormatInt(unixTime, 10)
	unixTimeMinutesString := strconv.FormatInt(unixTime/60, 10)

	testcases := []struct {
		name        string
		inputString string
		expected    string
	}{
		{
			name:        "Get CronJobControllerV2 or CronJob's Name with alphabet characters",
			inputString: "hello-name",
			expected:    "",
		},
		{
			name:        "Get CronJobControllerV2 or CronJob's Name with special characters and exact 10 characters",
			inputString: "hello-1678995&64",
			expected:    "",
		},
		{
			name:        "Get CronJobControllerV2 or CronJob's Name with Unix Time not equal to 10 letters",
			inputString: "hello-238",
			expected:    "",
		},
		{
			name:        "Get CronJobControllerV2's Name after k8s v1.21 with correct Unix Time",
			inputString: "hello-" + unixTimeMinutesString,
			expected:    "hello",
		},
		{
			name:        "Get CronJobControllerV2's Name after k8s v1.21 with alphabet Unix Time",
			inputString: "hello-" + unixTimeMinutesString + "a28bc",
			expected:    "",
		},

		{
			name:        "Get CronJobControllerV2's Name after k8s v1.21 with Unix Time not equal to 10 letters",
			inputString: "hello" + unixTimeMinutesString + "523",
			expected:    "",
		},
		{
			name:        "Get CronJob's Name before k8s v1.21 with correct Unix Time",
			inputString: "hello-" + unixTimeString,
			expected:    "hello",
		},
		{
			name:        "Get CronJob's Name before k8s v1.21 with special characters",
			inputString: "hello-" + unixTimeString + "&#64",
			expected:    "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, parseCronJobFromJob(tc.inputString), tc.expected)
		})
	}
}
