// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nodemetadatacache

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	leasePrefix = "cwagent-node-metadata-"

	annotationHostID = "cwagent.amazonaws.com/host.id"
	annotationHostName = "cwagent.amazonaws.com/host.name"
	annotationHostType = "cwagent.amazonaws.com/host.type"
	annotationImageID  = "cwagent.amazonaws.com/host.image.id"
	annotationAZ       = "cwagent.amazonaws.com/cloud.availability_zone"
)

// NodeMetadata holds the IMDS-resolved host attributes for a single node,
// extracted from a cwagent-node-metadata-* Lease's annotations.
type NodeMetadata struct {
	HostID           string
	HostName         string
	HostType         string
	HostImageID      string
	AvailabilityZone string
	RenewTime        time.Time
	LeaseDuration    int32
}

// NodeMetadataCache is an OTel extension that watches Kubernetes Leases in a
// configured namespace and maintains an in-memory cache of per-node host metadata.
// The nodemetadataenricher processor uses this cache to enrich KSM metrics.
type NodeMetadataCache struct {
	logger *zap.Logger
	config *Config
	cache  map[string]*NodeMetadata
	mutex  sync.RWMutex
	stopCh chan struct{}
}

var _ extension.Extension = (*NodeMetadataCache)(nil)

// SetForTest populates the cache with test data. Exported for cross-package test use.
func (c *NodeMetadataCache) SetForTest(nodeName string, metadata *NodeMetadata) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[nodeName] = metadata
}

// NewForTest creates a NodeMetadataCache suitable for unit testing. Exported for cross-package test use.
func NewForTest(logger *zap.Logger) *NodeMetadataCache {
	return &NodeMetadataCache{
		logger: logger,
		config: &Config{Namespace: "amazon-cloudwatch"},
		cache:  make(map[string]*NodeMetadata),
	}
}

// Get returns the cached NodeMetadata for the given node name, or nil if the
// node is not in the cache or the Lease is stale (renewTime + leaseDuration < now).
func (c *NodeMetadataCache) Get(nodeName string) *NodeMetadata {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	entry, ok := c.cache[nodeName]
	if !ok {
		return nil
	}
	// Check staleness: renewTime + leaseDuration must be >= now
	expiry := entry.RenewTime.Add(time.Duration(entry.LeaseDuration) * time.Second)
	if time.Now().After(expiry) {
		return nil
	}
	return entry
}

// Start creates a K8s clientset and starts a Lease informer scoped to the
// configured namespace. Informer event handlers populate the cache.
func (c *NodeMetadataCache) Start(_ context.Context, _ component.Host) error {
	c.logger.Info("Starting nodemetadatacache extension",
		zap.String("namespace", c.config.Namespace),
	)

	config, err := rest.InClusterConfig()
	if err != nil {
		c.logger.Error("Failed to create in-cluster K8s config — nodemetadatacache will be empty", zap.Error(err))
		return nil // degrade gracefully — cache stays empty, enricher passes metrics through
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		c.logger.Error("Failed to create K8s clientset — nodemetadatacache will be empty", zap.Error(err))
		return nil // degrade gracefully
	}

	c.stopCh = make(chan struct{})

	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		5*time.Minute, // resync period — safety net for missed watch events
		informers.WithNamespace(c.config.Namespace),
	)

	leaseInformer := factory.Coordination().V1().Leases().Informer()
	leaseInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onLeaseAdd,
		UpdateFunc: c.onLeaseUpdate,
		DeleteFunc: c.onLeaseDelete,
	})

	factory.Start(c.stopCh)
	factory.WaitForCacheSync(c.stopCh)

	c.logger.Info("nodemetadatacache extension started, Lease informer synced")
	return nil
}

// Shutdown stops the informer and clears the cache.
func (c *NodeMetadataCache) Shutdown(_ context.Context) error {
	if c.stopCh != nil {
		close(c.stopCh)
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache = make(map[string]*NodeMetadata)
	c.logger.Info("nodemetadatacache extension shut down")
	return nil
}

func (c *NodeMetadataCache) onLeaseAdd(obj interface{}) {
	lease, ok := obj.(*coordinationv1.Lease)
	if !ok {
		return
	}
	c.handleLeaseEvent(lease)
}

func (c *NodeMetadataCache) onLeaseUpdate(_, newObj interface{}) {
	lease, ok := newObj.(*coordinationv1.Lease)
	if !ok {
		return
	}
	c.handleLeaseEvent(lease)
}

func (c *NodeMetadataCache) onLeaseDelete(obj interface{}) {
	lease, ok := obj.(*coordinationv1.Lease)
	if !ok {
		// Handle deleted final state unknown (tombstone)
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		lease, ok = tombstone.Obj.(*coordinationv1.Lease)
		if !ok {
			return
		}
	}

	if !strings.HasPrefix(lease.Name, leasePrefix) {
		return
	}

	nodeName := strings.TrimPrefix(lease.Name, leasePrefix)
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.cache, nodeName)
	c.logger.Debug("Removed node metadata cache entry", zap.String("node", nodeName))
}

// handleLeaseEvent processes a Lease add or update event. It extracts the five
// annotation values and stores them in the cache keyed by node name.
func (c *NodeMetadataCache) handleLeaseEvent(lease *coordinationv1.Lease) {
	if !strings.HasPrefix(lease.Name, leasePrefix) {
		return
	}

	nodeName := strings.TrimPrefix(lease.Name, leasePrefix)
	annotations := lease.Annotations

	// All five annotations must be present
	hostID, ok1 := annotations[annotationHostID]
	hostName, ok2 := annotations[annotationHostName]
	hostType, ok3 := annotations[annotationHostType]
	imageID, ok4 := annotations[annotationImageID]
	az, ok5 := annotations[annotationAZ]

	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		c.logger.Warn("Skipping Lease with missing annotations",
			zap.String("lease", lease.Name),
		)
		return
	}

	var renewTime time.Time
	if lease.Spec.RenewTime != nil {
		renewTime = lease.Spec.RenewTime.Time
	} else {
		c.logger.Warn("Skipping Lease with missing renewTime",
			zap.String("lease", lease.Name),
		)
		return
	}

	var leaseDuration int32
	if lease.Spec.LeaseDurationSeconds != nil {
		leaseDuration = *lease.Spec.LeaseDurationSeconds
	} else {
		c.logger.Warn("Skipping Lease with missing leaseDurationSeconds",
			zap.String("lease", lease.Name),
		)
		return
	}

	entry := &NodeMetadata{
		HostID:           hostID,
		HostName:         hostName,
		HostType:         hostType,
		HostImageID:      imageID,
		AvailabilityZone: az,
		RenewTime:        renewTime,
		LeaseDuration:    leaseDuration,
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[nodeName] = entry
	c.logger.Debug("Updated node metadata cache entry",
		zap.String("node", nodeName),
		zap.String("hostID", hostID),
	)
}
