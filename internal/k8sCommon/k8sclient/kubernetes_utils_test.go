// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"testing"
)

func TestInferWorkloadName(t *testing.T) {
	testCases := []struct {
		name     string
		podName  string
		service  string
		expected string
	}{
		{
			name:     "StatefulSet single digit",
			podName:  "mysql-0",
			service:  "fallback-service",
			expected: "mysql",
		},
		{
			name:     "StatefulSet multiple digits",
			podName:  "mysql-10",
			service:  "fallback-service",
			expected: "mysql",
		},
		{
			name:     "ReplicaSet or DaemonSet bare pod (5 char suffix)",
			podName:  "nginx-b2dfg",
			service:  "fallback-service",
			expected: "nginx",
		},
		{
			name:     "Deployment-based ReplicaSet pod (two-level suffix)",
			podName:  "nginx-76977669dc-lwx64",
			service:  "fallback-service",
			expected: "nginx",
		},
		{
			name:     "Non matching, fallback to service",
			podName:  "simplepod",
			service:  "my-service",
			expected: "my-service",
		},
		{
			name:     "ReplicaSet name with some numeric part, still 5 char suffix",
			podName:  "nginx-123-d9stt",
			service:  "my-service",
			expected: "nginx-123",
		},
		{
			name:     "Confusing case but still matches a deployment-based RS suffix",
			podName:  "nginx-245678-d9stt",
			service:  "nginx-service",
			expected: "nginx",
		},
		{
			name:     "Confusing case not matching any known pattern, fallback to service if none matched fully",
			podName:  "nginx-123456-d9stt",
			service:  "nginx-service",
			expected: "nginx-123456",
		},
		{
			name:     "Empty Pod name, fallback to service",
			podName:  "",
			service:  "service",
			expected: "service",
		},
		{
			name:     "No match, empty fallback returns full pod name",
			podName:  "custom-app-xyz123",
			service:  "",
			expected: "custom-app-xyz123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := inferWorkloadName(tc.podName, tc.service)
			if got != tc.expected {
				t.Errorf("inferWorkloadName(%q, %q) = %q; want %q",
					tc.podName, tc.service, got, tc.expected)
			}
		})
	}
}
