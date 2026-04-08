// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	k8slease "github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/lease"
)

const (
	testNodeName  = "ip-10-0-1-42.ec2.internal"
	testNamespace = "amazon-cloudwatch"
)

// newTestEC2Info creates an EC2Info with pre-populated IMDS fields for testing.
func newTestEC2Info() *EC2Info {
	ei := &EC2Info{}
	ei.InstanceID = "i-0abc111def222ghi3"
	ei.AccountID = "123456789012"
	ei.InstanceType = "m5.xlarge"
	ei.ImageID = "ami-0123456789abcdef0"
	ei.AvailabilityZone = "us-east-1a"
	ei.Hostname = "ip-10-0-1-42.ec2.internal"
	return ei
}

func TestLeaseWriterBuildLease(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)
	lease := lw.buildLease()

	// Verify Lease name
	expectedName := "cwagent-node-metadata-" + testNodeName
	assert.Equal(t, expectedName, lease.Name)

	// Verify namespace
	assert.Equal(t, testNamespace, lease.Namespace)

	// Verify all five annotations
	assert.Equal(t, "i-0abc111def222ghi3", lease.Annotations[k8slease.AnnotationHostID])
	assert.Equal(t, "ip-10-0-1-42.ec2.internal", lease.Annotations[k8slease.AnnotationHostName])
	assert.Equal(t, "m5.xlarge", lease.Annotations[k8slease.AnnotationHostType])
	assert.Equal(t, "ami-0123456789abcdef0", lease.Annotations[k8slease.AnnotationImageID])
	assert.Equal(t, "us-east-1a", lease.Annotations[k8slease.AnnotationAZ])
	assert.Len(t, lease.Annotations, 5)

	// Verify leaseDurationSeconds
	require.NotNil(t, lease.Spec.LeaseDurationSeconds)
	assert.Equal(t, int32(7200), *lease.Spec.LeaseDurationSeconds)

	// Verify holderIdentity
	require.NotNil(t, lease.Spec.HolderIdentity)
	assert.Equal(t, expectedName, *lease.Spec.HolderIdentity)

	// Verify renewTime is set
	require.NotNil(t, lease.Spec.RenewTime)

	// Verify no ownerReferences
	assert.Empty(t, lease.OwnerReferences)
}

func TestLeaseWriterCreateLease(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)

	// Create the Lease via the K8s client
	lease := lw.buildLease()
	created, err := fakeClient.CoordinationV1().Leases(testNamespace).Create(
		context.Background(), lease, metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Verify the Lease exists in the fake API with correct fields
	assert.Equal(t, "cwagent-node-metadata-"+testNodeName, created.Name)
	assert.Equal(t, testNamespace, created.Namespace)
	assert.Equal(t, "i-0abc111def222ghi3", created.Annotations[k8slease.AnnotationHostID])
	assert.Equal(t, "ip-10-0-1-42.ec2.internal", created.Annotations[k8slease.AnnotationHostName])
	assert.Equal(t, "m5.xlarge", created.Annotations[k8slease.AnnotationHostType])
	assert.Equal(t, "ami-0123456789abcdef0", created.Annotations[k8slease.AnnotationImageID])
	assert.Equal(t, "us-east-1a", created.Annotations[k8slease.AnnotationAZ])

	// Fetch it back from the fake API to confirm persistence
	fetched, err := fakeClient.CoordinationV1().Leases(testNamespace).Get(
		context.Background(), "cwagent-node-metadata-"+testNodeName, metav1.GetOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, created.Name, fetched.Name)
	assert.Equal(t, int32(7200), *fetched.Spec.LeaseDurationSeconds)
}

func TestLeaseWriterRenewalUpdatesRenewTime(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)

	// Create the initial Lease
	lease := lw.buildLease()
	_, err := fakeClient.CoordinationV1().Leases(testNamespace).Create(
		context.Background(), lease, metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Record the initial renewTime
	initial, err := fakeClient.CoordinationV1().Leases(testNamespace).Get(
		context.Background(), lw.leaseName(), metav1.GetOptions{},
	)
	require.NoError(t, err)
	initialRenewTime := initial.Spec.RenewTime.Time

	// Small sleep to ensure time advances
	time.Sleep(10 * time.Millisecond)

	// Call the actual renewal method
	lw.renewLeaseWithRetry()

	// Verify renewTime changed
	updated, err := fakeClient.CoordinationV1().Leases(testNamespace).Get(
		context.Background(), lw.leaseName(), metav1.GetOptions{},
	)
	require.NoError(t, err)
	assert.True(t, updated.Spec.RenewTime.After(initialRenewTime),
		"renewTime should advance after renewal")
}

func TestLeaseWriterStopDeletesLease(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)

	// Pre-create the Lease so Stop() has something to delete
	lease := lw.buildLease()
	_, err := fakeClient.CoordinationV1().Leases(testNamespace).Create(
		context.Background(), lease, metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Verify it exists
	_, err = fakeClient.CoordinationV1().Leases(testNamespace).Get(
		context.Background(), lw.leaseName(), metav1.GetOptions{},
	)
	require.NoError(t, err)

	// Call Stop() — should best-effort delete the Lease
	lw.Stop()

	// Verify the Lease is deleted
	_, err = fakeClient.CoordinationV1().Leases(testNamespace).Get(
		context.Background(), lw.leaseName(), metav1.GetOptions{},
	)
	assert.Error(t, err, "Lease should be deleted after Stop()")
}

func TestLeaseWriterLeaseName(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	// Verify node name comes from constructor parameter
	lw := NewLeaseWriter(ec2Info, "my-custom-node", testNamespace, fakeClient.CoordinationV1(), logger)
	assert.Equal(t, "cwagent-node-metadata-my-custom-node", lw.leaseName())

	// Verify it does NOT use the IMDS hostname (which is different from the node name here)
	lw2 := NewLeaseWriter(ec2Info, "different-node", testNamespace, fakeClient.CoordinationV1(), logger)
	assert.Equal(t, "cwagent-node-metadata-different-node", lw2.leaseName())
	assert.NotEqual(t, "cwagent-node-metadata-"+ec2Info.GetHostname(), lw2.leaseName())
}

func TestLeaseWriterJitterWithinBounds(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)

	// Verify the jitterMax is set to 30 seconds
	assert.Equal(t, 30*time.Second, lw.jitterMax)

	// Run jitterSleep multiple times and verify it completes within bounds.
	// We set jitterMax to a small value to keep the test fast.
	lw.jitterMax = 50 * time.Millisecond
	for i := 0; i < 10; i++ {
		start := time.Now()
		lw.jitterSleep()
		elapsed := time.Since(start)
		assert.True(t, elapsed < 60*time.Millisecond,
			"jitterSleep should complete within jitterMax bounds, took %v", elapsed)
		assert.True(t, elapsed >= 0, "jitterSleep should not have negative duration")
	}
}

func TestLeaseWriterJitterZeroMax(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)
	lw.jitterMax = 0

	// Should return immediately without sleeping
	start := time.Now()
	lw.jitterSleep()
	elapsed := time.Since(start)
	assert.True(t, elapsed < 5*time.Millisecond, "jitterSleep with zero max should return immediately")
}

func TestLeaseWriterNoOwnerReferences(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)
	lease := lw.buildLease()

	// Create the Lease and verify no ownerReferences
	created, err := fakeClient.CoordinationV1().Leases(testNamespace).Create(
		context.Background(), lease, metav1.CreateOptions{},
	)
	require.NoError(t, err)
	assert.Empty(t, created.OwnerReferences, "Lease should not have ownerReferences")
}

func TestLeaseWriterDefaultValues(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)

	assert.Equal(t, int32(7200), lw.leaseDuration)
	assert.Equal(t, 1*time.Hour, lw.renewInterval)
	assert.Equal(t, 30*time.Second, lw.jitterMax)
	assert.NotNil(t, lw.done)
}
