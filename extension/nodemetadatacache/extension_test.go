// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nodemetadatacache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func testLease(nodeName string, renewTime time.Time, duration int32) *coordinationv1.Lease {
	now := metav1.NewMicroTime(renewTime)
	name := leasePrefix + nodeName
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "amazon-cloudwatch",
			Annotations: map[string]string{
				annotationHostID:   "i-0abc111",
				annotationHostName: "ip-10-0-1-42.ec2.internal",
				annotationHostType: "m5.xlarge",
				annotationImageID:  "ami-0123",
				annotationAZ:       "us-east-1a",
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &name,
			LeaseDurationSeconds: &duration,
			RenewTime:            &now,
		},
	}
}

func newTestCache() *NodeMetadataCache {
	logger, _ := zap.NewDevelopment()
	return &NodeMetadataCache{
		logger: logger,
		config: &Config{Namespace: "amazon-cloudwatch"},
		cache:  make(map[string]*NodeMetadata),
	}
}

func TestGetReturnsNilForUnknownNode(t *testing.T) {
	c := newTestCache()
	result := c.Get("unknown")
	assert.Nil(t, result, "Get should return nil for a node not in the cache")
}

func TestGetReturnsMetadataAfterAdd(t *testing.T) {
	c := newTestCache()
	lease := testLease("node-1", time.Now(), 300)
	c.handleLeaseEvent(lease)

	result := c.Get("node-1")
	require.NotNil(t, result, "Get should return metadata after add")
	assert.Equal(t, "i-0abc111", result.HostID)
	assert.Equal(t, "ip-10-0-1-42.ec2.internal", result.HostName)
	assert.Equal(t, "m5.xlarge", result.HostType)
	assert.Equal(t, "ami-0123", result.HostImageID)
	assert.Equal(t, "us-east-1a", result.AvailabilityZone)
}

func TestGetReturnsNilAfterDelete(t *testing.T) {
	c := newTestCache()
	lease := testLease("node-1", time.Now(), 300)
	c.handleLeaseEvent(lease)

	// Verify it's in the cache
	require.NotNil(t, c.Get("node-1"))

	// Simulate delete
	c.onLeaseDelete(lease)

	result := c.Get("node-1")
	assert.Nil(t, result, "Get should return nil after delete")
}

func TestGetReturnsNilForStaleLease(t *testing.T) {
	c := newTestCache()
	// renewTime 10 minutes ago, leaseDuration 300s (5 min) → expired 5 min ago
	staleTime := time.Now().Add(-10 * time.Minute)
	lease := testLease("node-1", staleTime, 300)
	c.handleLeaseEvent(lease)

	result := c.Get("node-1")
	assert.Nil(t, result, "Get should return nil for a stale Lease")
}

func TestGetReturnsMetadataForFreshLease(t *testing.T) {
	c := newTestCache()
	lease := testLease("node-1", time.Now(), 300)
	c.handleLeaseEvent(lease)

	result := c.Get("node-1")
	require.NotNil(t, result, "Get should return metadata for a fresh Lease")
	assert.Equal(t, "i-0abc111", result.HostID)
	assert.Equal(t, int32(300), result.LeaseDuration)
}

func TestIgnoresLeasesWithoutPrefix(t *testing.T) {
	c := newTestCache()
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-other-lease",
			Namespace: "amazon-cloudwatch",
			Annotations: map[string]string{
				annotationHostID:   "i-0abc111",
				annotationHostName: "ip-10-0-1-42.ec2.internal",
				annotationHostType: "m5.xlarge",
				annotationImageID:  "ami-0123",
				annotationAZ:       "us-east-1a",
			},
		},
		Spec: coordinationv1.LeaseSpec{},
	}
	c.handleLeaseEvent(lease)

	// Cache should be empty — the lease name doesn't have the prefix
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	assert.Empty(t, c.cache, "Cache should be empty for leases without the prefix")
}

func TestSkipsLeaseWithMissingAnnotations(t *testing.T) {
	c := newTestCache()
	now := metav1.NewMicroTime(time.Now())
	duration := int32(300)
	name := leasePrefix + "node-1"
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "amazon-cloudwatch",
			Annotations: map[string]string{
				// Only 3 of 5 annotations
				annotationHostID:   "i-0abc111",
				annotationHostName: "ip-10-0-1-42.ec2.internal",
				annotationHostType: "m5.xlarge",
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &name,
			LeaseDurationSeconds: &duration,
			RenewTime:            &now,
		},
	}
	c.handleLeaseEvent(lease)

	result := c.Get("node-1")
	assert.Nil(t, result, "Get should return nil when Lease has missing annotations")

	c.mutex.RLock()
	defer c.mutex.RUnlock()
	assert.Empty(t, c.cache, "Cache should be empty when Lease has missing annotations")
}

func TestConcurrentReadWrite(t *testing.T) {
	c := newTestCache()
	var wg sync.WaitGroup
	const goroutines = 50

	// Half the goroutines write, half read
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		if i%2 == 0 {
			go func(idx int) {
				defer wg.Done()
				lease := testLease("node-1", time.Now(), 300)
				c.handleLeaseEvent(lease)
			}(i)
		} else {
			go func(idx int) {
				defer wg.Done()
				_ = c.Get("node-1")
			}(i)
		}
	}

	wg.Wait()
	// If we get here without a race detector panic, the test passes
}

func TestDeleteHandlesTombstone(t *testing.T) {
	c := newTestCache()
	lease := testLease("node-1", time.Now(), 300)
	c.handleLeaseEvent(lease)
	require.NotNil(t, c.Get("node-1"))

	// Simulate a tombstone delete (DeletedFinalStateUnknown)
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "amazon-cloudwatch/cwagent-node-metadata-node-1",
		Obj: lease,
	}
	c.onLeaseDelete(tombstone)

	assert.Nil(t, c.Get("node-1"), "Get should return nil after tombstone delete")
}

func TestDeleteIgnoresLeasesWithoutPrefix(t *testing.T) {
	c := newTestCache()
	// Add a valid entry first
	lease := testLease("node-1", time.Now(), 300)
	c.handleLeaseEvent(lease)
	require.NotNil(t, c.Get("node-1"))

	// Try to delete a lease without the prefix — should not affect the cache
	otherLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-other-lease",
			Namespace: "amazon-cloudwatch",
		},
	}
	c.onLeaseDelete(otherLease)

	assert.NotNil(t, c.Get("node-1"), "Valid cache entry should not be affected by unrelated delete")
}

func TestUpdateOverwritesCacheEntry(t *testing.T) {
	c := newTestCache()
	lease := testLease("node-1", time.Now(), 300)
	c.handleLeaseEvent(lease)

	// Update with different annotations
	updatedLease := testLease("node-1", time.Now(), 300)
	updatedLease.Annotations[annotationHostType] = "c5.2xlarge"
	updatedLease.Annotations[annotationAZ] = "us-east-1b"
	c.handleLeaseEvent(updatedLease)

	result := c.Get("node-1")
	require.NotNil(t, result)
	assert.Equal(t, "c5.2xlarge", result.HostType, "HostType should be updated")
	assert.Equal(t, "us-east-1b", result.AvailabilityZone, "AZ should be updated")
	// Unchanged fields should remain
	assert.Equal(t, "i-0abc111", result.HostID)
}

func TestSkipsLeaseWithMissingRenewTime(t *testing.T) {
	c := newTestCache()
	duration := int32(300)
	name := leasePrefix + "node-1"
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "amazon-cloudwatch",
			Annotations: map[string]string{
				annotationHostID:   "i-0abc111",
				annotationHostName: "ip-10-0-1-42.ec2.internal",
				annotationHostType: "m5.xlarge",
				annotationImageID:  "ami-0123",
				annotationAZ:       "us-east-1a",
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &name,
			LeaseDurationSeconds: &duration,
			RenewTime:            nil, // missing
		},
	}
	c.handleLeaseEvent(lease)

	assert.Nil(t, c.Get("node-1"), "Get should return nil when Lease has no renewTime")
}

func TestSkipsLeaseWithMissingLeaseDuration(t *testing.T) {
	c := newTestCache()
	now := metav1.NewMicroTime(time.Now())
	name := leasePrefix + "node-1"
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "amazon-cloudwatch",
			Annotations: map[string]string{
				annotationHostID:   "i-0abc111",
				annotationHostName: "ip-10-0-1-42.ec2.internal",
				annotationHostType: "m5.xlarge",
				annotationImageID:  "ami-0123",
				annotationAZ:       "us-east-1a",
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &name,
			LeaseDurationSeconds: nil, // missing
			RenewTime:            &now,
		},
	}
	c.handleLeaseEvent(lease)

	assert.Nil(t, c.Get("node-1"), "Get should return nil when Lease has no leaseDurationSeconds")
}
