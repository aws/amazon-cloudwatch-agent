// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func (p *podWatcher) removeHostNetworkRecords(pod *corev1.Pod) {
	for _, port := range getHostNetworkPorts(pod) {
		p.deleter.DeleteWithDelay(p.ipToPod, pod.Status.HostIP+":"+port)
	}
}

func (p *podWatcher) handlePodAdd(pod *corev1.Pod) {
	if pod.Spec.HostNetwork && pod.Status.HostIP != "" {
		for _, port := range getHostNetworkPorts(pod) {
			p.ipToPod.Store(pod.Status.HostIP+":"+port, pod.Name)
		}
	}
	if pod.Status.PodIP != "" {
		p.ipToPod.Store(pod.Status.PodIP, pod.Name)
	}
}

func (p *podWatcher) handlePodUpdate(newPod *corev1.Pod, oldPod *corev1.Pod) {
	// HostNetwork is an immutable field
	if newPod.Spec.HostNetwork && oldPod.Status.HostIP != newPod.Status.HostIP {
		if oldPod.Status.HostIP != "" {
			p.logger.Debug("deleting host ip from cache", zap.String("hostNetwork", oldPod.Status.HostIP))
			p.removeHostNetworkRecords(oldPod)
		}
		if newPod.Status.HostIP != "" {
			for _, port := range getHostNetworkPorts(newPod) {
				p.ipToPod.Store(newPod.Status.HostIP+":"+port, newPod.Name)
			}
		}
	}
	if oldPod.Status.PodIP != newPod.Status.PodIP {
		if oldPod.Status.PodIP != "" {
			p.logger.Debug("deleting pod ip from cache", zap.String("podNetwork", oldPod.Status.PodIP))
			p.deleter.DeleteWithDelay(p.ipToPod, oldPod.Status.PodIP)
		}
		if newPod.Status.PodIP != "" {
			p.ipToPod.Store(newPod.Status.PodIP, newPod.Name)
		}
	}
}

func (p *podWatcher) onAddOrUpdatePod(pod, oldPod *corev1.Pod) {
	if oldPod == nil {
		p.handlePodAdd(pod)
	} else {
		p.handlePodUpdate(pod, oldPod)
	}

	workloadAndNamespace := getWorkloadAndNamespace(pod)

	if workloadAndNamespace != "" {
		p.podToWorkloadAndNamespace.Store(pod.Name, workloadAndNamespace)
		podLabels := mapset.NewSet[string]()
		for key, value := range pod.ObjectMeta.Labels {
			podLabels.Add(key + "=" + value)
		}
		if podLabels.Cardinality() > 0 {
			p.workloadAndNamespaceToLabels.Store(workloadAndNamespace, podLabels)
		}
		if oldPod == nil {
			p.workloadPodCount[workloadAndNamespace]++
			p.logger.Debug("Added pod", zap.String("pod", pod.Name), zap.String("workload", workloadAndNamespace), zap.Int("count", p.workloadPodCount[workloadAndNamespace]))
		}
	}
}

func (p *podWatcher) onDeletePod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	if pod.Spec.HostNetwork && pod.Status.HostIP != "" {
		p.logger.Debug("deleting host ip from cache", zap.String("hostNetwork", pod.Status.HostIP))
		p.removeHostNetworkRecords(pod)
	}
	if pod.Status.PodIP != "" {
		p.logger.Debug("deleting pod ip from cache", zap.String("podNetwork", pod.Status.PodIP))
		p.deleter.DeleteWithDelay(p.ipToPod, pod.Status.PodIP)
	}

	if workloadKey, ok := p.podToWorkloadAndNamespace.Load(pod.Name); ok {
		workloadAndNamespace := workloadKey.(string)
		p.workloadPodCount[workloadAndNamespace]--
		p.logger.Debug("decrementing pod count", zap.String("workload", workloadAndNamespace), zap.Int("podCount", p.workloadPodCount[workloadAndNamespace]))
		if p.workloadPodCount[workloadAndNamespace] == 0 {
			p.deleter.DeleteWithDelay(p.workloadAndNamespaceToLabels, workloadAndNamespace)
		}
	} else {
		p.logger.Error("failed to load pod workloadKey", zap.String("pod", pod.Name))
	}
	p.deleter.DeleteWithDelay(p.podToWorkloadAndNamespace, pod.Name)
}

type podWatcher struct {
	ipToPod                      *sync.Map
	podToWorkloadAndNamespace    *sync.Map
	workloadAndNamespaceToLabels *sync.Map
	workloadPodCount             map[string]int
	logger                       *zap.Logger
	informer                     cache.SharedIndexInformer
	deleter                      Deleter
}

func newPodWatcher(logger *zap.Logger, sharedInformerFactory informers.SharedInformerFactory, deleter Deleter) *podWatcher {
	podInformer := sharedInformerFactory.Core().V1().Pods().Informer()
	err := podInformer.SetTransform(minimizePod)
	if err != nil {
		logger.Error("failed to minimize Pod objects", zap.Error(err))
	}

	return &podWatcher{
		ipToPod:                      &sync.Map{},
		podToWorkloadAndNamespace:    &sync.Map{},
		workloadAndNamespaceToLabels: &sync.Map{},
		workloadPodCount:             make(map[string]int),
		logger:                       logger,
		informer:                     podInformer,
		deleter:                      deleter,
	}
}

func (p *podWatcher) run(stopCh chan struct{}) {
	p.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			p.logger.Debug("list and watch for pod: ADD " + pod.Name)
			p.onAddOrUpdatePod(pod, nil)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*corev1.Pod)
			oldPod := oldObj.(*corev1.Pod)
			p.logger.Debug("list and watch for pods: UPDATE " + pod.Name)
			p.onAddOrUpdatePod(pod, oldPod)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			p.logger.Debug("list and watch for pods: DELETE " + pod.Name)
			p.onDeletePod(obj)
		},
	})

	go p.informer.Run(stopCh)

}

func (p *podWatcher) waitForCacheSync(stopCh chan struct{}) {
	if !cache.WaitForNamedCacheSync("podWatcher", stopCh, p.informer.HasSynced) {
		p.logger.Fatal("timed out waiting for kubernetes pod watcher caches to sync")
	}

	p.logger.Info("podWatcher: Cache synced")
}

// minimizePod removes fields that could contain large objects, and retain essential
// fields needed for IP/name translation. The following fields must be kept:
// - ObjectMeta: Namespace, Name, Labels, OwnerReference
// - Spec: HostNetwork, ContainerPorts
// - Status: PodIP/s, HostIP/s
func minimizePod(obj interface{}) (interface{}, error) {
	if pod, ok := obj.(*corev1.Pod); ok {
		pod.Annotations = nil
		pod.Finalizers = nil
		pod.ManagedFields = nil

		pod.Spec.Volumes = nil
		pod.Spec.InitContainers = nil
		pod.Spec.EphemeralContainers = nil
		pod.Spec.ImagePullSecrets = nil
		pod.Spec.HostAliases = nil
		pod.Spec.SchedulingGates = nil
		pod.Spec.ResourceClaims = nil
		pod.Spec.Tolerations = nil
		pod.Spec.Affinity = nil

		pod.Status.InitContainerStatuses = nil
		pod.Status.ContainerStatuses = nil
		pod.Status.EphemeralContainerStatuses = nil

		for i := 0; i < len(pod.Spec.Containers); i++ {
			c := &pod.Spec.Containers[i]
			c.Image = ""
			c.Command = nil
			c.Args = nil
			c.EnvFrom = nil
			c.Env = nil
			c.Resources = corev1.ResourceRequirements{}
			c.VolumeMounts = nil
			c.VolumeDevices = nil
			c.SecurityContext = nil
		}
	}
	return obj, nil
}
