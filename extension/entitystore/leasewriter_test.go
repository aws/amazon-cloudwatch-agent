// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

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

func TestCreateLeaseWithRetry_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)

	success := lw.createLeaseWithRetry()
	assert.True(t, success, "createLeaseWithRetry should succeed on first try")

	// Verify the Lease exists in the fake API with correct fields
	fetched, err := fakeClient.CoordinationV1().Leases(testNamespace).Get(
		context.Background(), lw.leaseName(), metav1.GetOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, "cwagent-node-metadata-"+testNodeName, fetched.Name)
	assert.Equal(t, "i-0abc111def222ghi3", fetched.Annotations[k8slease.AnnotationHostID])
	assert.Equal(t, "ip-10-0-1-42.ec2.internal", fetched.Annotations[k8slease.AnnotationHostName])
	assert.Equal(t, "m5.xlarge", fetched.Annotations[k8slease.AnnotationHostType])
	assert.Equal(t, "ami-0123456789abcdef0", fetched.Annotations[k8slease.AnnotationImageID])
	assert.Equal(t, "us-east-1a", fetched.Annotations[k8slease.AnnotationAZ])
	assert.Equal(t, int32(7200), *fetched.Spec.LeaseDurationSeconds)
}

func TestCreateLeaseWithRetry_AlreadyExists(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)

	// Pre-create a Lease with stale annotations to simulate a leftover from a previous pod
	staleLease := lw.buildLease()
	staleLease.Annotations[k8slease.AnnotationHostType] = "t3.micro"
	staleLease.Annotations[k8slease.AnnotationAZ] = "us-west-2a"
	_, err := fakeClient.CoordinationV1().Leases(testNamespace).Create(
		context.Background(), staleLease, metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// createLeaseWithRetry should adopt the existing Lease via Get+Update
	success := lw.createLeaseWithRetry()
	assert.True(t, success, "createLeaseWithRetry should succeed by adopting existing Lease")

	// Verify annotations were updated to current EC2Info values
	fetched, err := fakeClient.CoordinationV1().Leases(testNamespace).Get(
		context.Background(), lw.leaseName(), metav1.GetOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, "m5.xlarge", fetched.Annotations[k8slease.AnnotationHostType], "should be updated from stale t3.micro")
	assert.Equal(t, "us-east-1a", fetched.Annotations[k8slease.AnnotationAZ], "should be updated from stale us-west-2a")
}

func TestCreateLeaseWithRetry_StopsOnDoneChannel(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ec2Info := newTestEC2Info()
	fakeClient := fake.NewSimpleClientset()

	// Make Create always fail with a transient error so the retry loop runs
	fakeClient.PrependReactor("create", "leases", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("api server unavailable")
	})

	lw := NewLeaseWriter(ec2Info, testNodeName, testNamespace, fakeClient.CoordinationV1(), logger)

	// Close done channel before calling — the first Create fails, backoff select sees done closed
	close(lw.done)

	success := lw.createLeaseWithRetry()
	assert.False(t, success, "createLeaseWithRetry should return false when done channel is closed")
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

	// Record the initial renewTime via DeepCopy (no time.Sleep needed)
	initial, err := fakeClient.CoordinationV1().Leases(testNamespace).Get(
		context.Background(), lw.leaseName(), metav1.GetOptions{},
	)
	require.NoError(t, err)
	initialRenewTime := initial.Spec.RenewTime.DeepCopy()

	// Call the actual renewal method
	lw.renewLeaseWithRetry()

	// Verify renewTime changed
	updated, err := fakeClient.CoordinationV1().Leases(testNamespace).Get(
		context.Background(), lw.leaseName(), metav1.GetOptions{},
	)
	require.NoError(t, err)
	assert.True(t, !updated.Spec.RenewTime.Before(initialRenewTime),
		"renewTime should be >= initial after renewal")
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

	// Single call with generous bound to avoid flakiness under CI load.
	lw.jitterMax = 50 * time.Millisecond
	start := time.Now()
	lw.jitterSleep()
	elapsed := time.Since(start)
	assert.True(t, elapsed < 500*time.Millisecond,
		"jitterSleep should complete well within bounds, took %v", elapsed)
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
