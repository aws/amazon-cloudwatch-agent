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
		logger:                        zap.NewNop(),
		ipToPodMetadata:               &sync.Map{},
		serviceNamespaceToPodMetadata: &sync.Map{},
		deleter:                       mockDeleter,
	}
}

// createTestEndpointSlice is a helper to build a minimal EndpointSlice.
// The slice will have one Endpoint (with its TargetRef) and a list of Ports.
// svcName is stored in the Labels (key "kubernetes.io/service-name") if non-empty.
func createTestEndpointSlice(uid, namespace, svcName, podName string, addresses []string, portNumbers []int32) *discv1.EndpointSlice {
	// Build the port list.
	var ports []discv1.EndpointPort
	for i, p := range portNumbers {
		portVal := p // need a pointer
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
// the appropriate keys are inserted into the maps.
func TestEndpointSliceAddition(t *testing.T) {
	watcher := newEndpointSliceWatcherForTest()

	// Create a test EndpointSlice:
	//   - UID: "uid-1", Namespace: "testns"
	//   - Labels: "kubernetes.io/service-name" = "mysvc"
	//   - One Endpoint with TargetRef.Kind "Pod", Name "workload-69dww", Namespace "testns"
	//   - Endpoint.Addresses: ["1.2.3.4"]
	//   - One Port with value 80.
	slice := createTestEndpointSlice("uid-1", "testns", "mysvc", "workload-69dww", []string{"1.2.3.4"}, []int32{80})

	// Call the add handler.
	watcher.handleSliceAdd(slice)

	// The dummy InferWorkloadName returns "workload", so full workload becomes {Workload: "workload", Namespace: "testns", Node: ""}
	expectedVal := PodMetadata{Workload: "workload", Namespace: "testns", Node: ""}

	// We expect the following keys:
	// - For the endpoint: "1.2.3.4" and "1.2.3.4:80"
	// - From the service label: "mysvc@testns"
	var expectedIPKeys = []string{"1.2.3.4", "1.2.3.4:80"}
	var expectedSvcKeys = []string{"mysvc@testns"}

	// Verify ipToPodMetadata.
	for _, key := range expectedIPKeys {
		val, ok := watcher.ipToPodMetadata.Load(key)
		assert.True(t, ok, "expected ipToPodMetadata key %s", key)
		assert.Equal(t, expectedVal, val, "ipToPodMetadata[%s] mismatch", key)
	}

	// Verify serviceNamespaceToPodMetadata.
	for _, key := range expectedSvcKeys {
		val, ok := watcher.serviceNamespaceToPodMetadata.Load(key)
		assert.True(t, ok, "expected serviceNamespaceToPodMetadata key %s", key)
		assert.Equal(t, expectedVal, val, "serviceNamespaceToPodMetadata[%s] mismatch", key)
	}

	// Verify that sliceToKeysMap recorded all keys.
	val, ok := watcher.sliceToKeysMap.Load(string(slice.UID))
	assert.True(t, ok, "expected sliceToKeysMap to contain UID %s", slice.UID)
	keysIface := val.([]string)
	// Sort for comparison.
	sort.Strings(keysIface)
	allExpected := append(expectedIPKeys, expectedSvcKeys...)
	sort.Strings(allExpected)
	assert.Equal(t, allExpected, keysIface, "sliceToKeysMap keys mismatch")
}

// TestEndpointSliceDeletion verifies that when an EndpointSlice is deleted,
// all keys that were added are removed.
func TestEndpointSliceDeletion(t *testing.T) {
	watcher := newEndpointSliceWatcherForTest()

	// Create a test EndpointSlice (same as addition test).
	slice := createTestEndpointSlice("uid-1", "testns", "mysvc", "workload-76977669dc-lwx64", []string{"1.2.3.4"}, []int32{80})
	watcher.handleSliceAdd(slice)

	// Now call deletion.
	watcher.handleSliceDelete(slice)

	// Verify that the keys are removed from ipToPodMetadata.
	removedKeys := []string{"1.2.3.4", "1.2.3.4:80", "mysvc@testns"}
	for _, key := range removedKeys {
		_, ok := watcher.ipToPodMetadata.Load(key)
		_, okSvc := watcher.serviceNamespaceToPodMetadata.Load(key)
		assert.False(t, ok, "expected ipToPodMetadata key %s to be deleted", key)
		assert.False(t, okSvc, "expected serviceNamespaceToPodMetadata key %s to be deleted", key)
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
		// One endpoint with TargetRef Name "workload-75d9d5968d-fx8px", Addresses ["1.2.3.4"], Port 80.
		oldSlice := createTestEndpointSlice("uid-2", "testns", "mysvc", "workload-75d9d5968d-fx8px", []string{"1.2.3.4"}, []int32{80})
		watcher.handleSliceAdd(oldSlice)

		// New slice: same UID, but svc label changed to "othersvc"
		// and a different endpoint: TargetRef Name "workload-6d9b7f8597-wbvxn", Addresses ["1.2.3.5"], Port 443.
		newSlice := createTestEndpointSlice("uid-2", "testns", "othersvc", "workload-6d9b7f8597-wbvxn", []string{"1.2.3.5"}, []int32{443})

		// Call update handler.
		watcher.handleSliceUpdate(oldSlice, newSlice)

		expectedVal := PodMetadata{Workload: "workload", Namespace: "testns", Node: ""}

		// Old keys that should be removed:
		// "1.2.3.4" and "1.2.3.4:80" and service key "mysvc@testns"
		removedKeys := []string{"1.2.3.4", "1.2.3.4:80", "mysvc@testns"}
		for _, key := range removedKeys {
			_, ok := watcher.ipToPodMetadata.Load(key)
			_, okSvc := watcher.serviceNamespaceToPodMetadata.Load(key)
			assert.False(t, ok, "expected ipToPodMetadata key %s to be removed", key)
			assert.False(t, okSvc, "expected serviceNamespaceToPodMetadata key %s to be removed", key)
		}

		// New keys that should be added:
		// "1.2.3.5", "1.2.3.5:443", and service key "othersvc@testns"
		addedKeys := []string{"1.2.3.5", "1.2.3.5:443", "othersvc@testns"}
		for _, key := range addedKeys {
			var val interface{}
			var ok bool
			// For service key, check serviceNamespaceToPodMetadata; for others, check ipToPodMetadata.
			if key == "othersvc@testns" {
				val, ok = watcher.serviceNamespaceToPodMetadata.Load(key)
			} else {
				val, ok = watcher.ipToPodMetadata.Load(key)
			}
			assert.True(t, ok, "expected key %s to be added", key)
			assert.Equal(t, expectedVal, val, "value for key %s mismatch", key)
		}

		// Check that sliceToKeysMap now contains exactly the new keys.
		val, ok := watcher.sliceToKeysMap.Load(string(newSlice.UID))
		assert.True(t, ok, "expected sliceToKeysMap entry for UID %s", newSlice.UID)
		gotKeys := val.([]string)
		sort.Strings(gotKeys)
		expectedKeys := []string{"1.2.3.5", "1.2.3.5:443", "othersvc@testns"}
		sort.Strings(expectedKeys)
		assert.True(t, reflect.DeepEqual(expectedKeys, gotKeys), "sliceToKeysMap keys mismatch, got: %v, want: %v", gotKeys, expectedKeys)
	})

	// --- Subtest: Partial overlap ---
	t.Run("partial overlap", func(t *testing.T) {
		watcher := newEndpointSliceWatcherForTest()

		// Old slice: UID "uid-3", Namespace "testns", svc label "mysvc",
		// with one endpoint: TargetRef "workload-6d9b7f8597-b5l2j", Addresses ["1.2.3.4"], Port 80.
		oldSlice := createTestEndpointSlice("uid-3", "testns", "mysvc", "workload-6d9b7f8597-b5l2j", []string{"1.2.3.4"}, []int32{80})
		watcher.handleSliceAdd(oldSlice)

		// New slice: same UID, same svc label ("mysvc") but now two endpoints.
		// First endpoint: same as before: Addresses ["1.2.3.4"], Port 80.
		// Second endpoint: Addresses ["1.2.3.5"], Port 80.
		// (Since svc label remains, the service key "mysvc@testns" remains the same.)
		// We expect the new keys to be the union of:
		//   From first endpoint: "1.2.3.4", "1.2.3.4:80"
		//   From second endpoint: "1.2.3.5", "1.2.3.5:80"
		//   And the service key "mysvc@testns".
		name := "port-0"
		protocol := v1.ProtocolTCP
		newSlice := &discv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "uid-3", // same UID
				Namespace: "testns",
				Labels: map[string]string{
					"kubernetes.io/service-name": "mysvc",
				},
			},
			// Two endpoints.
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
			// Single port: 80.
			Ports: []discv1.EndpointPort{
				{
					Name:     &name,
					Protocol: &protocol,
					Port:     func() *int32 { p := int32(80); return &p }(),
				},
			},
		}

		// Call update handler.
		watcher.handleSliceUpdate(oldSlice, newSlice)

		expectedVal := PodMetadata{Workload: "workload", Namespace: "testns", Node: ""}
		// Expected keys now:
		// From endpoint 1: "1.2.3.4", "1.2.3.4:80"
		// From endpoint 2: "1.2.3.5", "1.2.3.5:80"
		// And service key: "mysvc@testns"
		expectedKeysIP := []string{"1.2.3.4", "1.2.3.4:80", "1.2.3.5", "1.2.3.5:80"}
		expectedKeysSvc := []string{"mysvc@testns"}

		// Verify that all expected keys are present.
		for _, key := range expectedKeysIP {
			val, ok := watcher.ipToPodMetadata.Load(key)
			assert.True(t, ok, "expected ipToPodMetadata key %s", key)
			assert.Equal(t, expectedVal, val, "ipToPodMetadata[%s] mismatch", key)
		}
		for _, key := range expectedKeysSvc {
			val, ok := watcher.serviceNamespaceToPodMetadata.Load(key)
			assert.True(t, ok, "expected serviceNamespaceToPodMetadata key %s", key)
			assert.Equal(t, expectedVal, val, "serviceNamespaceToPodMetadata[%s] mismatch", key)
		}

		// And check that sliceToKeysMap contains the union of the keys.
		val, ok := watcher.sliceToKeysMap.Load("uid-3")
		assert.True(t, ok, "expected sliceToKeysMap to contain uid-3")
		gotKeys := val.([]string)
		allExpected := append(expectedKeysIP, expectedKeysSvc...)
		sort.Strings(gotKeys)
		sort.Strings(allExpected)
		assert.True(t, reflect.DeepEqual(allExpected, gotKeys), "sliceToKeysMap keys mismatch, got: %v, want: %v", gotKeys, allExpected)
	})
}

func TestEndpointSliceWithEmptyFields(t *testing.T) {
	t.Run("empty namespace", func(t *testing.T) {
		watcher := newEndpointSliceWatcherForTest()

		// Create a test EndpointSlice with empty namespace
		slice := createTestEndpointSlice("uid-1", "", "mysvc", "workload-69dww", []string{"1.2.3.4"}, []int32{80})

		// Call the add handler
		watcher.handleSliceAdd(slice)

		// Verify that no service entries were added since namespace is empty
		_, ok := watcher.serviceNamespaceToPodMetadata.Load("mysvc@")
		assert.False(t, ok, "expected no serviceNamespaceToPodMetadata entry when namespace is empty")
	})

	t.Run("empty service name", func(t *testing.T) {
		watcher := newEndpointSliceWatcherForTest()

		// Create a test EndpointSlice with empty service name
		slice := createTestEndpointSlice("uid-2", "testns", "", "workload-69dww", []string{"1.2.3.4"}, []int32{80})

		// Call the add handler
		watcher.handleSliceAdd(slice)

		// Verify that no service entries were added since service name is empty
		_, ok := watcher.serviceNamespaceToPodMetadata.Load("@testns")
		assert.False(t, ok, "expected no serviceNamespaceToPodMetadata entry when service name is empty")
	})
}
