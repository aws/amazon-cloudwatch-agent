// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestAttachNamespace function
func TestAttachNamespace(t *testing.T) {
	result := attachNamespace("testResource", "testNamespace")
	if result != "testResource@testNamespace" {
		t.Errorf("attachNamespace was incorrect, got: %s, want: %s.", result, "testResource@testNamespace")
	}
}

// TestGetServiceAndNamespace function
func TestGetServiceAndNamespace(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testService",
			Namespace: "testNamespace",
		},
	}
	result := getServiceAndNamespace(service)
	if result != "testService@testNamespace" {
		t.Errorf("getServiceAndNamespace was incorrect, got: %s, want: %s.", result, "testService@testNamespace")
	}
}

// TestExtractResourceAndNamespace function
func TestExtractResourceAndNamespace(t *testing.T) {
	// Test normal case
	name, namespace := ExtractResourceAndNamespace("testService@testNamespace")
	if name != "testService" || namespace != "testNamespace" {
		t.Errorf("extractResourceAndNamespace was incorrect, got: %s and %s, want: %s and %s.", name, namespace, "testService", "testNamespace")
	}

	// Test invalid case
	name, namespace = ExtractResourceAndNamespace("invalid")
	if name != "" || namespace != "" {
		t.Errorf("extractResourceAndNamespace was incorrect, got: %s and %s, want: %s and %s.", name, namespace, "", "")
	}
}

func TestExtractWorkloadNameFromRS(t *testing.T) {
	testCases := []struct {
		name           string
		replicaSetName string
		want           string
		shouldErr      bool
	}{
		{
			name:           "Valid ReplicaSet Name",
			replicaSetName: "my-deployment-5859ffc7ff",
			want:           "my-deployment",
			shouldErr:      false,
		},
		{
			name:           "Invalid ReplicaSet Name - No Hyphen",
			replicaSetName: "mydeployment5859ffc7ff",
			want:           "",
			shouldErr:      true,
		},
		{
			name:           "Invalid ReplicaSet Name - Less Than 10 Suffix Characters",
			replicaSetName: "my-deployment-bc2",
			want:           "",
			shouldErr:      true,
		},
		{
			name:           "Invalid ReplicaSet Name - More Than 10 Suffix Characters",
			replicaSetName: "my-deployment-5859ffc7ffx",
			want:           "",
			shouldErr:      true,
		},
		{
			name:           "Invalid ReplicaSet Name - Invalid Characters in Suffix",
			replicaSetName: "my-deployment-aeiou12345",
			want:           "",
			shouldErr:      true,
		},
		{
			name:           "Invalid ReplicaSet Name - Empty String",
			replicaSetName: "",
			want:           "",
			shouldErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractWorkloadNameFromRS(tc.replicaSetName)

			if (err != nil) != tc.shouldErr {
				t.Errorf("extractWorkloadNameFromRS() error = %v, wantErr %v", err, tc.shouldErr)
				return
			}

			if got != tc.want {
				t.Errorf("extractWorkloadNameFromRS() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractWorkloadNameFromPodName(t *testing.T) {
	testCases := []struct {
		name      string
		podName   string
		want      string
		shouldErr bool
	}{
		{
			name:      "Valid Pod Name",
			podName:   "my-replicaset-bc24f",
			want:      "my-replicaset",
			shouldErr: false,
		},
		{
			name:      "Invalid Pod Name - No Hyphen",
			podName:   "myreplicasetbc24f",
			want:      "",
			shouldErr: true,
		},
		{
			name:      "Invalid Pod Name - Less Than 5 Suffix Characters",
			podName:   "my-replicaset-bc2",
			want:      "",
			shouldErr: true,
		},
		{
			name:      "Invalid Pod Name - More Than 5 Suffix Characters",
			podName:   "my-replicaset-bc24f5",
			want:      "",
			shouldErr: true,
		},
		{
			name:      "Invalid Pod Name - Empty String",
			podName:   "",
			want:      "",
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractWorkloadNameFromPodName(tc.podName)

			if (err != nil) != tc.shouldErr {
				t.Errorf("extractWorkloadNameFromPodName() error = %v, wantErr %v", err, tc.shouldErr)
				return
			}

			if got != tc.want {
				t.Errorf("extractWorkloadNameFromPodName() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestGetWorkloadAndNamespace function
func TestGetWorkloadAndNamespace(t *testing.T) {
	// Test ReplicaSet case
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testPod",
			Namespace: "testNamespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "testDeployment-5d68bc5f49",
				},
			},
		},
	}
	result := GetWorkloadAndNamespace(pod)
	if result != "testDeployment@testNamespace" {
		t.Errorf("getDeploymentAndNamespace was incorrect, got: %s, want: %s.", result, "testDeployment@testNamespace")
	}

	// Test StatefulSet case
	pod.OwnerReferences[0].Kind = "StatefulSet"
	pod.OwnerReferences[0].Name = "testStatefulSet"
	result = GetWorkloadAndNamespace(pod)
	if result != "testStatefulSet@testNamespace" {
		t.Errorf("getWorkloadAndNamespace was incorrect, got: %s, want: %s.", result, "testStatefulSet@testNamespace")
	}

	// Test Other case
	pod.OwnerReferences[0].Kind = "Other"
	pod.OwnerReferences[0].Name = "testOther"
	result = GetWorkloadAndNamespace(pod)
	if result != "" {
		t.Errorf("getWorkloadAndNamespace was incorrect, got: %s, want: %s.", result, "")
	}

	// Test no OwnerReferences case
	pod.OwnerReferences = nil
	result = GetWorkloadAndNamespace(pod)
	if result != "" {
		t.Errorf("getWorkloadAndNamespace was incorrect, got: %s, want: %s.", result, "")
	}
}

func TestExtractIPPort(t *testing.T) {
	// Test valid IP:Port
	ip, port, ok := ExtractIPPort("192.0.2.0:8080")
	assert.Equal(t, "192.0.2.0", ip)
	assert.Equal(t, "8080", port)
	assert.True(t, ok)

	// Test invalid IP:Port
	ip, port, ok = ExtractIPPort("192.0.2:8080")
	assert.Equal(t, "", ip)
	assert.Equal(t, "", port)
	assert.False(t, ok)

	// Test IP only
	ip, port, ok = ExtractIPPort("192.0.2.0")
	assert.Equal(t, "", ip)
	assert.Equal(t, "", port)
	assert.False(t, ok)
}

func TestInferWorkloadName(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		service  string
		expected string
	}{
		{"StatefulSet single digit", "mysql-0", "service", "mysql"},
		{"StatefulSet multiple digits", "mysql-10", "service", "mysql"},
		{"ReplicaSet bare pod", "nginx-b2dfg", "service", "nginx"},
		{"Deployment-based ReplicaSet pod", "nginx-76977669dc-lwx64", "service", "nginx"},
		{"Non matching", "simplepod", "service", "service"},
		{"ReplicaSet name with number suffix", "nginx-123-d9stt", "service", "nginx-123"},
		{"Some confusing case with a replicaSet/daemonset name matching the pattern", "nginx-245678-d9stt", "nginx-service", "nginx"},
		// when the regex pattern doesn't matter, we just fall back to service name to handle all the edge cases
		{"Some confusing case with a replicaSet/daemonset name not matching the pattern", "nginx-123456-d9stt", "nginx-service", "nginx-123456"},
		{"Empty", "", "service", "service"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := InferWorkloadName(tc.input, tc.service)
			if got != tc.expected {
				t.Errorf("InferWorkloadName(%q) = %q; expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

// UUIDString wraps a string with a unique identifier.
type UUIDString struct {
	value string
	uuid  string
}

// NewUUIDString returns a new UUIDString instance with a generated UUID.
func NewUUIDString(s string) UUIDString {
	return UUIDString{
		value: s,
		uuid:  uuid.NewString(),
	}
}

// String returns the string value and implements the fmt.Stringer interface.
func (u UUIDString) String() string {
	return u.value
}

func (u UUIDString) UUID() string {
	return u.uuid
}

// TestDeleteWithDelay_NoUpdate verifies that if the value is not updated,
// the key is deleted after the delay.
func TestTimedDeleterWithIDCheck_DeleteWithDelay_NoUpdate(t *testing.T) {
	m := &sync.Map{}
	key := "testKey"
	initialVal := NewUUIDString("value")
	m.Store(key, initialVal)

	// Use a short delay to make the test run quickly.
	td := TimedDeleterWithIDCheck{Delay: 10 * time.Millisecond}
	td.DeleteWithDelay(m, key)

	// Wait for longer than the deletion delay.
	time.Sleep(20 * time.Millisecond)

	if _, ok := m.Load(key); ok {
		t.Errorf("Expected key %q to be deleted, but it still exists", key)
	}
}

// TestDeleteWithDelay_WithUpdate verifies that if the value is updated before the deletion delay expires,
// the key is not deleted.
func TestTimedDeleterWithIDCheck_DeleteWithDelay_WithUpdate(t *testing.T) {
	m := &sync.Map{}
	key := "testKey"
	initialVal := NewUUIDString("value")
	m.Store(key, initialVal)

	td := TimedDeleterWithIDCheck{Delay: 20 * time.Millisecond}
	td.DeleteWithDelay(m, key)

	// Wait a bit before updating (less than td.Delay).
	time.Sleep(10 * time.Millisecond)
	updatedVal := NewUUIDString("value") // same content, but a new instance (different UUID)
	m.Store(key, updatedVal)

	// Wait long enough for the deletion delay to expire.
	time.Sleep(20 * time.Millisecond)

	if _, ok := m.Load(key); !ok {
		t.Errorf("Expected key %q to remain after update, but it was deleted", key)
	}
}

// TestDeleteWithDelay_InvalidType verifies that if the value stored is not a UUIDString,
// no deletion occurs.
func TestTimedDeleterWithIDCheck_DeleteWithDelay_InvalidType(t *testing.T) {
	m := &sync.Map{}
	key := "invalidKey"
	// Store a plain string instead of UUIDString.
	m.Store(key, "a simple string")

	td := TimedDeleterWithIDCheck{Delay: 10 * time.Millisecond}
	td.DeleteWithDelay(m, key)

	time.Sleep(20 * time.Millisecond)
	if _, ok := m.Load(key); !ok {
		t.Errorf("Expected key %q to remain since value is not a UUIDString, but it was deleted", key)
	}
}
