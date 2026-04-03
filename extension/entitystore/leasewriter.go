// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	coordinationv1 "k8s.io/api/coordination/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
)

const (
	leasePrefix              = "cwagent-node-metadata-"
	annotationHostID         = "cwagent.amazonaws.com/host.id"
	annotationHostName       = "cwagent.amazonaws.com/host.name"
	annotationHostType       = "cwagent.amazonaws.com/host.type"
	annotationImageID        = "cwagent.amazonaws.com/host.image.id"
	annotationAZ             = "cwagent.amazonaws.com/cloud.availability_zone"
	defaultLeaseDuration     = int32(7200) // 2 hours
	defaultRenewInterval     = 1 * time.Hour
	leaseJitterMax           = 30 * time.Second
	leaseBackoffInitial      = 1 * time.Second
	leaseBackoffMax          = 30 * time.Second
	leaseEC2InfoPollInterval = 5 * time.Second
)

// LeaseWriter creates and renews a Kubernetes Lease containing IMDS-resolved
// node metadata as annotations. The cluster-scraper's NodeMetadataCache watches
// these Leases to enrich KSM metrics with correct per-node host attributes.
type LeaseWriter struct {
	ec2Info       *EC2Info
	nodeName      string
	namespace     string
	client        coordinationv1client.LeasesGetter
	logger        *zap.Logger
	done          chan struct{}
	wg            sync.WaitGroup
	leaseDuration int32
	renewInterval time.Duration
	jitterMax     time.Duration
}

// NewLeaseWriter creates a new LeaseWriter. The nodeName should come from the
// K8S_NODE_NAME env var (Kubernetes downward API), not from IMDS hostname.
func NewLeaseWriter(ec2Info *EC2Info, nodeName, namespace string, client coordinationv1client.LeasesGetter, logger *zap.Logger) *LeaseWriter {
	return &LeaseWriter{
		ec2Info:       ec2Info,
		nodeName:      nodeName,
		namespace:     namespace,
		client:        client,
		logger:        logger,
		done:          make(chan struct{}),
		leaseDuration: defaultLeaseDuration,
		renewInterval: defaultRenewInterval,
		jitterMax:     leaseJitterMax,
	}
}

// Start begins the Lease lifecycle in a goroutine: jitter → wait for EC2Info → create → renew loop.
func (lw *LeaseWriter) Start() {
	lw.wg.Add(1)
	go lw.run()
}

// Stop stops the renewal goroutine, waits for it to exit, then performs a
// best-effort delete of the Lease.
func (lw *LeaseWriter) Stop() {
	close(lw.done)
	lw.wg.Wait()
	lw.deleteLease()
}

func (lw *LeaseWriter) run() {
	defer lw.wg.Done()

	// Random jitter to prevent thundering herd on cluster-wide DaemonSet rollouts
	lw.jitterSleep()

	// Wait for EC2Info to be populated
	if !lw.waitForEC2Info() {
		return
	}

	// Create the initial Lease
	if !lw.createLeaseWithRetry() {
		return
	}

	// Renewal loop
	ticker := time.NewTicker(lw.renewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-lw.done:
			return
		case <-ticker.C:
			lw.renewLeaseWithRetry()
		}
	}
}

func (lw *LeaseWriter) jitterSleep() {
	if lw.jitterMax <= 0 {
		return
	}
	jitter := time.Duration(rand.Int63n(int64(lw.jitterMax))) // nolint:gosec
	select {
	case <-lw.done:
		return
	case <-time.After(jitter):
	}
}

func (lw *LeaseWriter) waitForEC2Info() bool {
	for {
		if lw.ec2Info.GetInstanceID() != "" {
			return true
		}
		select {
		case <-lw.done:
			return false
		case <-time.After(leaseEC2InfoPollInterval):
		}
	}
}

// leaseName returns the Lease object name for this node.
func (lw *LeaseWriter) leaseName() string {
	return leasePrefix + lw.nodeName
}

// buildAnnotations returns the IMDS metadata annotations for the Lease.
func (lw *LeaseWriter) buildAnnotations() map[string]string {
	return map[string]string{
		annotationHostID:   lw.ec2Info.GetInstanceID(),
		annotationHostName: lw.ec2Info.GetHostname(),
		annotationHostType: lw.ec2Info.GetInstanceType(),
		annotationImageID:  lw.ec2Info.GetImageID(),
		annotationAZ:       lw.ec2Info.GetAvailabilityZone(),
	}
}

// buildLease constructs the Lease object with IMDS metadata annotations.
// Extracted as a helper for unit testing.
func (lw *LeaseWriter) buildLease() *coordinationv1.Lease {
	now := metav1.NewMicroTime(time.Now())
	name := lw.leaseName()
	duration := lw.leaseDuration
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   lw.namespace,
			Annotations: lw.buildAnnotations(),
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &name,
			LeaseDurationSeconds: &duration,
			RenewTime:            &now,
		},
	}
}

func (lw *LeaseWriter) createLeaseWithRetry() bool {
	backoff := leaseBackoffInitial
	for {
		lease := lw.buildLease()
		_, err := lw.client.Leases(lw.namespace).Create(context.Background(), lease, metav1.CreateOptions{})
		if err == nil {
			lw.logger.Info("Created node metadata Lease",
				zap.String("name", lw.leaseName()),
				zap.String("namespace", lw.namespace),
			)
			return true
		}
		if k8serrors.IsAlreadyExists(err) {
			// Lease left over from a previous pod — adopt it via direct update.
			lw.logger.Info("Node metadata Lease already exists, adopting via update",
				zap.String("name", lw.leaseName()),
			)
			existing, getErr := lw.client.Leases(lw.namespace).Get(context.Background(), lw.leaseName(), metav1.GetOptions{})
			if getErr != nil {
				lw.logger.Error("Failed to get existing Lease for adoption", zap.Error(getErr))
				continue
			}
			now := metav1.NewMicroTime(time.Now())
			existing.Spec.RenewTime = &now
			duration := lw.leaseDuration
			existing.Spec.LeaseDurationSeconds = &duration
			existing.Annotations = lw.buildAnnotations()
			if _, updateErr := lw.client.Leases(lw.namespace).Update(context.Background(), existing, metav1.UpdateOptions{}); updateErr != nil {
				lw.logger.Error("Failed to adopt existing Lease", zap.Error(updateErr))
				continue
			}
			return true
		}
		lw.logger.Error("Failed to create node metadata Lease, retrying",
			zap.Error(err),
			zap.Duration("backoff", backoff),
		)
		select {
		case <-lw.done:
			return false
		case <-time.After(backoff):
		}
		backoff = backoff * 2
		if backoff > leaseBackoffMax {
			backoff = leaseBackoffMax
		}
	}
}

func (lw *LeaseWriter) renewLeaseWithRetry() {
	backoff := leaseBackoffInitial
	deadline := time.After(lw.renewInterval)
	for {
		// GET the existing Lease to obtain resourceVersion for optimistic concurrency.
		existing, err := lw.client.Leases(lw.namespace).Get(context.Background(), lw.leaseName(), metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// Lease was externally deleted — re-create it.
				lw.logger.Info("Node metadata Lease not found during renewal, re-creating",
					zap.String("name", lw.leaseName()),
				)
				lw.createLeaseWithRetry()
				return
			}
			lw.logger.Warn("Failed to get node metadata Lease for renewal, retrying",
				zap.Error(err),
				zap.Duration("backoff", backoff),
			)
			select {
			case <-lw.done:
				return
			case <-deadline:
				lw.logger.Warn("Renewal deadline exceeded, will retry on next tick",
					zap.String("name", lw.leaseName()))
				return
			case <-time.After(backoff):
			}
			backoff = backoff * 2
			if backoff > leaseBackoffMax {
				backoff = leaseBackoffMax
			}
			continue
		}

		// Update renewTime, annotations, and leaseDuration on the existing object (preserves resourceVersion).
		now := metav1.NewMicroTime(time.Now())
		existing.Spec.RenewTime = &now
		duration := lw.leaseDuration
		existing.Spec.LeaseDurationSeconds = &duration
		existing.Annotations = lw.buildAnnotations()

		_, err = lw.client.Leases(lw.namespace).Update(context.Background(), existing, metav1.UpdateOptions{})
		if err == nil {
			lw.logger.Debug("Renewed node metadata Lease", zap.String("name", lw.leaseName()))
			return
		}
		if k8serrors.IsNotFound(err) {
			lw.logger.Info("Node metadata Lease not found during update, re-creating",
				zap.String("name", lw.leaseName()),
			)
			lw.createLeaseWithRetry()
			return
		}
		lw.logger.Warn("Failed to renew node metadata Lease, retrying",
			zap.Error(err),
			zap.Duration("backoff", backoff),
		)
		select {
		case <-lw.done:
			return
		case <-deadline:
			lw.logger.Warn("Renewal deadline exceeded, will retry on next tick",
				zap.String("name", lw.leaseName()))
			return
		case <-time.After(backoff):
		}
		backoff = backoff * 2
		if backoff > leaseBackoffMax {
			backoff = leaseBackoffMax
		}
	}
}

func (lw *LeaseWriter) deleteLease() {
	err := lw.client.Leases(lw.namespace).Delete(context.Background(), lw.leaseName(), metav1.DeleteOptions{})
	if err != nil {
		lw.logger.Warn("Best-effort Lease delete failed (will expire via TTL)",
			zap.String("name", lw.leaseName()),
			zap.Error(err),
		)
		return
	}
	lw.logger.Info("Deleted node metadata Lease on shutdown", zap.String("name", lw.leaseName()))
}
