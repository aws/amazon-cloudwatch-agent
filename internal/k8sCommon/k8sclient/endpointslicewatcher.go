// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
	discv1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// EndpointSliceWatcher watches EndpointSlices and builds:
//  1. ip/ip:port -> {"workload", "namespace", "node"}
type EndpointSliceWatcher struct {
	logger          *zap.Logger
	informer        cache.SharedIndexInformer
	IPToPodMetadata *sync.Map // key: "ip" or "ip:port", val: {"workload", "namespace", "node"}

	// For bookkeeping, so we can remove old mappings upon EndpointSlice deletion
	sliceToKeysMap sync.Map // map[sliceUID string] -> []string of keys we inserted, which are "ip" or "ip:port"
	deleter        Deleter
}

// PodMetadata holds {"workload", "namespace", "node"}
type PodMetadata struct {
	Workload  string
	Namespace string
	Node      string
}

// kvPair holds one mapping from key -> value. The isService flag
// indicates whether this key is for a Service or for an IP/IP:port.
type kvPair struct {
	key   string      // key: "ip" or "ip:port"
	value PodMetadata // value: {"workload", "namespace", "node"}
}

// NewEndpointSliceWatcher creates an EndpointSlice watcher
func NewEndpointSliceWatcher(
	logger *zap.Logger,
	factory informers.SharedInformerFactory,
	deleter Deleter,
) *EndpointSliceWatcher {

	esInformer := factory.Discovery().V1().EndpointSlices().Informer()
	err := esInformer.SetTransform(minimizeEndpointSlice)
	if err != nil {
		logger.Error("failed to minimize Service objects", zap.Error(err))
	}

	return &EndpointSliceWatcher{
		logger:          logger,
		informer:        esInformer,
		IPToPodMetadata: &sync.Map{},
		deleter:         deleter,
	}
}

// run starts the endpointSliceWatcher.
func (w *EndpointSliceWatcher) Run(stopCh chan struct{}) {
	w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			w.handleSliceAdd(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			w.handleSliceUpdate(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			w.handleSliceDelete(obj)
		},
	})
	go w.informer.Run(stopCh)
}

func (w *EndpointSliceWatcher) WaitForCacheSync(stopCh chan struct{}) {
	if !cache.WaitForNamedCacheSync("endpointSliceWatcher", stopCh, w.informer.HasSynced) {
		w.logger.Error("timed out waiting for endpointSliceWatcher cache to sync")
	}
	w.logger.Info("endpointSliceWatcher: Cache synced")
}

// extractEndpointSliceKeyValuePairs computes the relevant mappings from an EndpointSlice.
//
// It returns a list of kvPair:
//   - All IP and IP:port keys  -> {"workload", "namespace", "node"}
//
// This function does NOT modify ipToPodMetadata It's purely for computing
// the pairs, so it can be reused by both add and update methods.
func (w *EndpointSliceWatcher) extractEndpointSliceKeyValuePairs(slice *discv1.EndpointSlice) []kvPair {
	var pairs []kvPair
	svcName := slice.Labels["kubernetes.io/service-name"]

	for _, endpoint := range slice.Endpoints {
		if endpoint.TargetRef != nil {
			if endpoint.TargetRef.Kind != "Pod" {
				continue
			}

			podName := endpoint.TargetRef.Name
			ns := endpoint.TargetRef.Namespace

			var nodeName string
			if endpoint.NodeName != nil {
				nodeName = *endpoint.NodeName
			}

			w.logger.Debug("Processing endpoint")

			derivedWorkload := inferWorkloadName(podName, svcName)
			if derivedWorkload == "" {
				w.logger.Warn("failed to infer workload name from Pod name")
				continue
			}
			fullWl :=
				PodMetadata{
					Workload:  derivedWorkload,
					Namespace: ns,
					Node:      nodeName,
				}

			// Build IP and IP:port pairs
			for _, addr := range endpoint.Addresses {
				// "ip" -> {"workload", "namespace", "node"}
				pairs = append(pairs, kvPair{
					key:   addr,
					value: fullWl,
				})

				// "ip:port" -> {"workload", "namespace", "node"} for each port
				for _, portDef := range slice.Ports {
					if portDef.Port != nil {
						ipPort := fmt.Sprintf("%s:%d", addr, *portDef.Port)
						pairs = append(pairs, kvPair{
							key:   ipPort,
							value: fullWl,
						})
					}
				}
			}
		}

	}

	return pairs
}

// handleSliceAdd handles a new EndpointSlice that wasn't seen before.
// It computes all keys and directly stores them. Then it records those keys
// in sliceToKeysMap so that we can remove them later upon deletion.
func (w *EndpointSliceWatcher) handleSliceAdd(obj interface{}) {
	newSlice := obj.(*discv1.EndpointSlice)
	w.logger.Debug("Received EndpointSlice Add",
		zap.String("uid", string(newSlice.UID)),
	)
	sliceUID := string(newSlice.UID)

	// Compute all key-value pairs for this new slice
	pairs := w.extractEndpointSliceKeyValuePairs(newSlice)
	w.logger.Debug("Extracted pairs from new slice",
		zap.Int("pairsCount", len(pairs)),
	)

	// Insert them into our ipToWorkload / serviceToWorkload, and track the keys.
	keys := make([]string, 0, len(pairs))
	for _, kv := range pairs {
		w.IPToPodMetadata.Store(kv.key, kv.value)
		keys = append(keys, kv.key)
	}

	// Save these keys so we can remove them on delete
	w.sliceToKeysMap.Store(sliceUID, keys)
}

// handleSliceUpdate handles an update from oldSlice -> newSlice.
// Instead of blindly removing all old keys and adding new ones, it diffs them:
//   - remove only keys that no longer exist,
//   - add only new keys that didn't exist before,
//   - keep those that haven't changed.
func (w *EndpointSliceWatcher) handleSliceUpdate(oldObj, newObj interface{}) {
	oldSlice := oldObj.(*discv1.EndpointSlice)
	newSlice := newObj.(*discv1.EndpointSlice)

	w.logger.Debug("Received EndpointSlice Update",
		zap.String("oldSliceUID", string(oldSlice.UID)),
		zap.String("newSliceUID", string(newSlice.UID)),
	)

	oldUID := string(oldSlice.UID)
	newUID := string(newSlice.UID)

	// 1) Fetch old keys from sliceToKeysMap (if present).
	var oldKeys []string
	if val, ok := w.sliceToKeysMap.Load(oldUID); ok {
		oldKeys = val.([]string)
	}

	// 2) Compute fresh pairs (and thus keys) from the new slice.
	newPairs := w.extractEndpointSliceKeyValuePairs(newSlice)
	var newKeys []string
	for _, kv := range newPairs {
		newKeys = append(newKeys, kv.key)
	}

	// Convert oldKeys/newKeys to sets for easy diff
	oldKeysSet := make(map[string]struct{}, len(oldKeys))
	for _, k := range oldKeys {
		oldKeysSet[k] = struct{}{}
	}
	newKeysSet := make(map[string]struct{}, len(newKeys))
	for _, k := range newKeys {
		newKeysSet[k] = struct{}{}
	}

	// 3) For each key in oldKeys that doesn't exist in newKeys, remove it
	for k := range oldKeysSet {
		if _, stillPresent := newKeysSet[k]; !stillPresent {
			w.deleter.DeleteWithDelay(w.IPToPodMetadata, k)
		}
	}

	// 4) For each key in newKeys that wasn't in oldKeys, we need to store it
	//    in the appropriate sync.Map. We'll look up the value from newPairs.
	for _, kv := range newPairs {
		if _, alreadyHad := oldKeysSet[kv.key]; !alreadyHad {
			w.IPToPodMetadata.Store(kv.key, kv.value)
		}
	}

	// 5) Update sliceToKeysMap for the new slice UID
	//    (Often the UID doesn't change across updates, but we'll handle it properly.)
	w.sliceToKeysMap.Delete(oldUID)
	w.sliceToKeysMap.Store(newUID, newKeys)

	w.logger.Debug("Finished handling EndpointSlice Update",
		zap.String("sliceUID", string(newSlice.UID)))
}

// handleSliceDelete removes any IP->workload or service->workload keys that were created by this slice.
func (w *EndpointSliceWatcher) handleSliceDelete(obj interface{}) {
	slice := obj.(*discv1.EndpointSlice)
	w.logger.Debug("Received EndpointSlice Delete",
		zap.String("uid", string(slice.UID)),
	)
	w.removeSliceKeys(slice)
}

func (w *EndpointSliceWatcher) removeSliceKeys(slice *discv1.EndpointSlice) {
	sliceUID := string(slice.UID)
	val, ok := w.sliceToKeysMap.Load(sliceUID)
	if !ok {
		return
	}

	keys := val.([]string)
	for _, k := range keys {
		w.deleter.DeleteWithDelay(w.IPToPodMetadata, k)
	}
	w.sliceToKeysMap.Delete(sliceUID)
}

// minimizeEndpointSlice removes fields that are not required by our mapping logic,
// retaining only the minimal set of fields needed (ObjectMeta.Name, Namespace, UID, Labels,
// Endpoints (with their Addresses and TargetRef) and Ports).
func minimizeEndpointSlice(obj interface{}) (interface{}, error) {
	eps, ok := obj.(*discv1.EndpointSlice)
	if !ok {
		return obj, fmt.Errorf("object is not an EndpointSlice")
	}

	// Minimize metadata: we only really need Name, Namespace, UID and Labels.
	eps.Annotations = nil
	eps.ManagedFields = nil
	eps.Finalizers = nil

	// The watcher only uses:
	// - eps.Labels["kubernetes.io/service-name"]
	// - eps.Namespace (from metadata)
	// - eps.UID (from metadata)
	// - eps.Endpoints: for each endpoint, its Addresses and TargetRef.
	// - eps.Ports: each port's Port (and optionally Name/Protocol)
	//
	// For each endpoint, clear fields that we don’t use.
	for i := range eps.Endpoints {
		// We only need Addresses and TargetRef. Hostname, and Zone are not used.
		eps.Endpoints[i].Hostname = nil
		eps.Endpoints[i].Zone = nil
		eps.Endpoints[i].DeprecatedTopology = nil
		eps.Endpoints[i].Hints = nil
	}

	// No transformation is needed for eps.Ports because we use them directly.
	return eps, nil
}
