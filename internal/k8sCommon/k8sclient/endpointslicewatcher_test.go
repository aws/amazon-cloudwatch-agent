// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	discv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// MockDeleter deletes a key immediately, useful for testing.
type MockDeleter struct{}

func (md *MockDeleter) DeleteWithDelay(m *sync.Map, key interface{}) {
	m.Delete(key)
}

var mockDeleter = &MockDeleter{}

func newEndpointSliceWatcherForTest() *EndpointSliceWatcher {
	return &EndpointSliceWatcher{
		logger:          zap.NewNop(),
		IPToPodMetadata: &sync.Map{},
		deleter:         mockDeleter,
	}
}

// createTestEndpointSlice is a helper to build a minimal EndpointSlice.
// The slice will have one Endpoint (with its TargetRef) and a list of Ports.
// svcName is stored in the Labels (key "kubernetes.io/service-name") if non-empty.
func createTestEndpointSlice(uid, namespace, svcName, podName string, addresses []string, portNumbers []int32, nodeName *string) *discv1.EndpointSlice {
	// Build the port list.
	var ports []discv1.EndpointPort
	for i, p := range portNumbers {
		portVal := p
		name := fmt.Sprintf("port-%d", i)
		protocol := v1.ProtocolTCP
		ports = append(ports, discv1.EndpointPort{
			Name:     &name,
			Protocol: &protocol,
			Port:     &portVal,
		})
	}

	// Build a single endpoint with the given addresses and a TargetRef.
	endpoint := discv1.Endpoint{
		Addresses: addresses,
		TargetRef: &v1.ObjectReference{
			Kind:      "Pod",
			Name:      podName,
			Namespace: namespace,
		},
	}
	if nodeName != nil {
		endpoint.NodeName = nodeName
	}

	labels := map[string]string{}
	if svcName != "" {
		labels["kubernetes.io/service-name"] = svcName
	}

	return &discv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID(uid),
			Namespace: namespace,
			Labels:    labels,
		},
		Endpoints: []discv1.Endpoint{endpoint},
		Ports:     ports,
	}
}

// --- Tests ---

// TestEndpointSliceAddition verifies that when a new EndpointSlice is added,
// the appropriate IP/IP:port -> PodMetadata entries are inserted into IPToPodMetadata.
func TestEndpointSliceAddition(t *testing.T) {
	watcher := newEndpointSliceWatcherForTest()

	// Create a test EndpointSlice:
	//   - UID: "uid-1", Namespace: "testns"
	//   - Labels: "kubernetes.io/service-name" = "mysvc"
	//   - One Endpoint with TargetRef.Kind = "Pod", Name "workload-69dww", Namespace "testns"
	//   - Endpoint.Addresses: ["1.2.3.4"]
	//   - One Port with value 80
	slice := createTestEndpointSlice("uid-1", "testns", "mysvc", "workload-69dww", []string{"1.2.3.4"}, []int32{80}, nil)

	// Call the add handler.
	watcher.handleSliceAdd(slice)

	// The code calls inferWorkloadName(podName, svcName) to get the Workload name.
	// For "workload-69dww", it typically infers "workload".
	// So expected PodMetadata is:
	expectedMeta := PodMetadata{
		Workload:  "workload",
		Namespace: "testns",
		Node:      "",
	}

	// We expect these keys to be present in IPToPodMetadata:
	//   - "1.2.3.4"
	//   - "1.2.3.4:80"
	var expectedKeys = []string{"1.2.3.4", "1.2.3.4:80"}

	for _, key := range expectedKeys {
		val, ok := watcher.IPToPodMetadata.Load(key)
		assert.True(t, ok, "expected IPToPodMetadata key %s", key)
		assert.Equal(t, expectedMeta, val, "IPToPodMetadata[%s] mismatch", key)
	}

	// Verify that sliceToKeysMap recorded all keys.
	val, ok := watcher.sliceToKeysMap.Load(string(slice.UID))
	assert.True(t, ok, "expected sliceToKeysMap to contain UID %s", slice.UID)
	storedKeys := val.([]string)
	sort.Strings(storedKeys)
	sort.Strings(expectedKeys)
	assert.Equal(t, expectedKeys, storedKeys, "sliceToKeysMap keys mismatch")
}

// TestEndpointSliceDeletion verifies that when an EndpointSlice is deleted,
// all keys that were added get removed from IPToPodMetadata.
func TestEndpointSliceDeletion(t *testing.T) {
	watcher := newEndpointSliceWatcherForTest()

	// Create a test EndpointSlice (similar to the addition test).
	slice := createTestEndpointSlice("uid-1", "testns", "mysvc", "workload-76977669dc-lwx64",
		[]string{"1.2.3.4"}, []int32{80}, nil)
	watcher.handleSliceAdd(slice)

	// Now call deletion.
	watcher.handleSliceDelete(slice)

	// Verify that the keys are removed from IPToPodMetadata.
	removedKeys := []string{"1.2.3.4", "1.2.3.4:80"}
	for _, key := range removedKeys {
		_, ok := watcher.IPToPodMetadata.Load(key)
		assert.False(t, ok, "expected IPToPodMetadata key %s to be deleted", key)
	}

	// Also verify that sliceToKeysMap no longer contains an entry.
	_, ok := watcher.sliceToKeysMap.Load(string(slice.UID))
	assert.False(t, ok, "expected sliceToKeysMap entry for UID %s to be deleted", slice.UID)
}

// TestEndpointSliceUpdate verifies that on updates, keys are added and/or removed as appropriate.
func TestEndpointSliceUpdate(t *testing.T) {
	// --- Subtest: Complete change (no overlap) ---
	t.Run("complete change", func(t *testing.T) {
		watcher := newEndpointSliceWatcherForTest()

		// Old slice:
		// UID "uid-2", Namespace "testns", svc label "mysvc",
		// One endpoint with Pod name "workload-75d9d5968d-fx8px", addresses ["1.2.3.4"], port 80.
		oldSlice := createTestEndpointSlice("uid-2", "testns", "mysvc", "workload-75d9d5968d-fx8px",
			[]string{"1.2.3.4"}, []int32{80}, nil)
		watcher.handleSliceAdd(oldSlice)

		// New slice: same UID, but label changed to "othersvc"
		// and a different endpoint: Pod "workload-6d9b7f8597-wbvxn", addresses ["1.2.3.5"], port 443.
		newSlice := createTestEndpointSlice("uid-2", "testns", "othersvc", "workload-6d9b7f8597-wbvxn",
			[]string{"1.2.3.5"}, []int32{443}, nil)

		// Call update handler.
		watcher.handleSliceUpdate(oldSlice, newSlice)

		// Old keys that should be removed:
		// "1.2.3.4" and "1.2.3.4:80"
		removedKeys := []string{"1.2.3.4", "1.2.3.4:80"}
		for _, key := range removedKeys {
			_, ok := watcher.IPToPodMetadata.Load(key)
			assert.False(t, ok, "expected IPToPodMetadata key %s to be removed", key)
		}

		// New keys that should be added:
		// "1.2.3.5", "1.2.3.5:443"
		// The derived workload name is "workload" (from "workload-6d9b7f8597-wbvxn"), namespace "testns".
		expectedMeta := PodMetadata{
			Workload:  "workload",
			Namespace: "testns",
			Node:      "",
		}
		addedKeys := []string{"1.2.3.5", "1.2.3.5:443"}
		for _, key := range addedKeys {
			val, ok := watcher.IPToPodMetadata.Load(key)
			assert.True(t, ok, "expected IPToPodMetadata key %s to be added", key)
			assert.Equal(t, expectedMeta, val, "value for key %s mismatch", key)
		}

		// Check that sliceToKeysMap now contains exactly the new keys.
		val, ok := watcher.sliceToKeysMap.Load(string(newSlice.UID))
		assert.True(t, ok, "expected sliceToKeysMap entry for UID %s", newSlice.UID)
		gotKeys := val.([]string)
		sort.Strings(gotKeys)
		sort.Strings(addedKeys)
		assert.True(t, reflect.DeepEqual(addedKeys, gotKeys),
			"sliceToKeysMap keys mismatch, got: %v, want: %v", gotKeys, addedKeys)
	})

	// --- Subtest: Partial overlap ---
	t.Run("partial overlap", func(t *testing.T) {
		watcher := newEndpointSliceWatcherForTest()

		// Old slice:
		// UID "uid-3", namespace "testns", label "mysvc",
		// 1 endpoint: Pod "workload-6d9b7f8597-b5l2j", addresses ["1.2.3.4"], port 80.
		oldSlice := createTestEndpointSlice("uid-3", "testns", "mysvc", "workload-6d9b7f8597-b5l2j",
			[]string{"1.2.3.4"}, []int32{80}, nil)
		watcher.handleSliceAdd(oldSlice)

		// New slice: same UID, same label "mysvc",
		// but now 2 endpoints:
		//   - Pod "workload-6d9b7f8597-b5l2j", addresses ["1.2.3.4"], port 80  (same as old)
		//   - Pod "workload-6d9b7f8597-fx8px", addresses ["1.2.3.5"], port 80 (new)
		newSlice := &discv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "uid-3",
				Namespace: "testns",
				Labels: map[string]string{
					"kubernetes.io/service-name": "mysvc",
				},
			},
			Endpoints: []discv1.Endpoint{
				{
					Addresses: []string{"1.2.3.4"},
					TargetRef: &v1.ObjectReference{
						Kind:      "Pod",
						Name:      "workload-6d9b7f8597-b5l2j",
						Namespace: "testns",
					},
				},
				{
					Addresses: []string{"1.2.3.5"},
					TargetRef: &v1.ObjectReference{
						Kind:      "Pod",
						Name:      "workload-6d9b7f8597-fx8px",
						Namespace: "testns",
					},
				},
			},
			Ports: []discv1.EndpointPort{
				{
					Name:     func() *string { s := "port-0"; return &s }(),
					Protocol: func() *v1.Protocol { p := v1.ProtocolTCP; return &p }(),
					Port:     func() *int32 { p := int32(80); return &p }(),
				},
			},
		}

		watcher.handleSliceUpdate(oldSlice, newSlice)

		// We expect:
		//   - "1.2.3.4" & "1.2.3.4:80" remain
		//   - "1.2.3.5" & "1.2.3.5:80" get added
		expectedMeta := PodMetadata{
			Workload:  "workload", // from "workload-6d9b7f8597-..."
			Namespace: "testns",
			Node:      "",
		}

		expectedKeys := []string{
			"1.2.3.4",
			"1.2.3.4:80",
			"1.2.3.5",
			"1.2.3.5:80",
		}
		for _, key := range expectedKeys {
			val, ok := watcher.IPToPodMetadata.Load(key)
			assert.True(t, ok, "expected IPToPodMetadata key %s", key)
			assert.Equal(t, expectedMeta, val, "IPToPodMetadata[%s] mismatch", key)
		}

		// Check that sliceToKeysMap has the union of all keys.
		val, ok := watcher.sliceToKeysMap.Load("uid-3")
		assert.True(t, ok, "expected sliceToKeysMap entry for uid-3")
		gotKeys := val.([]string)
		sort.Strings(gotKeys)
		sort.Strings(expectedKeys)
		assert.True(t, reflect.DeepEqual(expectedKeys, gotKeys),
			"sliceToKeysMap keys mismatch, got: %v, want: %v", gotKeys, expectedKeys)
	})
}
