// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stores

import (
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	corev1 "k8s.io/api/core/v1"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/kubeletutil"
	"github.com/aws/amazon-cloudwatch-agent/internal/mapWithExpiry"
	"github.com/aws/amazon-cloudwatch-agent/profiler"
)

const (
	refreshInterval    = 30 * time.Second
	MeasurementsExpiry = 10 * time.Minute
	PodsExpiry         = 2 * time.Minute
	memoryKey          = "memory"
	cpuKey             = "cpu"
	splitRegexStr      = "\\.|-"
	kubeProxy          = "kube-proxy"
	ignoreAnnotation   = "aws.amazon.com/cloudwatch-agent-ignore"
)

var (
	re = regexp.MustCompile(splitRegexStr)
)

type cachedEntry struct {
	pod      corev1.Pod
	creation time.Time
}

type Owner struct {
	OwnerKind string `json:"owner_kind"`
	OwnerName string `json:"owner_name"`
}

type prevPodMeasurement struct {
	containersRestarts int
}

type prevContainerMeasurement struct {
	restarts int
}

type PodStore struct {
	cache            *mapWithExpiry.MapWithExpiry
	prevMeasurements map[string]*mapWithExpiry.MapWithExpiry //preMeasurements per each Type (Pod, Container, etc)
	kubeClient       *kubeletutil.KubeClient
	lastRefreshed    time.Time
	nodeInfo         *nodeInfo
	prefFullPodName  bool
	sync.Mutex
}

func NewPodStore(hostIP string, prefFullPodName bool) *PodStore {
	podStore := &PodStore{
		cache:            mapWithExpiry.NewMapWithExpiry(PodsExpiry),
		prevMeasurements: make(map[string]*mapWithExpiry.MapWithExpiry),
		kubeClient:       &kubeletutil.KubeClient{Port: KubeSecurePort, BearerToken: BearerToken, KubeIP: hostIP},
		nodeInfo:         newNodeInfo(),
		prefFullPodName:  prefFullPodName,
	}

	// Try to detect kubelet permission issue here
	if _, err := podStore.kubeClient.ListPods(); err != nil {
		log.Panicf("Cannot get pod from kubelet, err: %v", err)
	}

	return podStore
}

func (p *PodStore) getPrevMeasurement(metricType, metricKey string) (interface{}, bool) {
	prevMeasurement, ok := p.prevMeasurements[metricType]
	if !ok {
		return nil, false
	}

	content, ok := prevMeasurement.Get(metricKey)

	if !ok {
		return nil, false
	}

	return content, true
}

func (p *PodStore) setPrevMeasurement(metricType, metricKey string, content interface{}) {
	prevMeasurement, ok := p.prevMeasurements[metricType]
	if !ok {
		prevMeasurement = mapWithExpiry.NewMapWithExpiry(MeasurementsExpiry)
		p.prevMeasurements[metricType] = prevMeasurement
	}
	prevMeasurement.Set(metricKey, content)
}

func (p *PodStore) RefreshTick() {
	now := time.Now()
	if now.Sub(p.lastRefreshed) >= refreshInterval {
		p.refresh(now)
		// call cleanup every refresh cycle
		p.cleanup(now)
		p.lastRefreshed = now
	}
}

func (p *PodStore) Decorate(metric telegraf.Metric, kubernetesBlob map[string]interface{}) bool {
	tags := metric.Tags()
	p.decorateDiskDevice(metric, tags)

	if tags[MetricType] == TypeNode {
		p.decorateNode(metric)
	} else if _, ok := tags[K8sPodNameKey]; ok {
		podKey := createPodKeyFromMetric(tags)
		if podKey == "" {
			log.Printf("E! podKey is unavailable when decorating pod.")
			return false
		}

		entry := p.getCachedEntry(podKey)
		if entry == nil {
			log.Printf("I! no pod is found for %s, refresh the cache now...", podKey)
			p.refresh(time.Now())
			entry = p.getCachedEntry(podKey)
		}

		// If pod is still not found, insert a placeholder to avoid too many refresh
		if entry == nil {
			log.Printf("W! no pod is found after reading through kubelet, add a placeholder for %s", podKey)
			p.setCachedEntry(podKey, &cachedEntry{creation: time.Now()})
			return false
		}

		// Ignore if we're told to ignore
		if strings.EqualFold(entry.pod.ObjectMeta.Annotations[ignoreAnnotation], "true") {
			return false
		}

		// If the entry is not a placeholder, decorate the pod
		if entry.pod.Name != "" {
			p.decorateCpu(metric, tags, &entry.pod)
			p.decorateMem(metric, tags, &entry.pod)
			p.addStatus(metric, tags, &entry.pod)
			addContainerCount(metric, tags, &entry.pod)
			addContainerId(&entry.pod, tags, metric, kubernetesBlob)
			p.addPodOwnersAndPodName(metric, &entry.pod, kubernetesBlob)
			addLabels(&entry.pod, kubernetesBlob)
		} else {
			log.Printf("W! no pod information is found in podstore for pod %s", podKey)
			return false
		}
	}
	return true
}

func (p *PodStore) getCachedEntry(podKey string) *cachedEntry {
	p.Lock()
	defer p.Unlock()
	if content, ok := p.cache.Get(podKey); ok {
		return content.(*cachedEntry)
	}
	return nil
}

func (p *PodStore) setCachedEntry(podKey string, entry *cachedEntry) {
	p.Lock()
	defer p.Unlock()
	p.cache.Set(podKey, entry)
}

func (p *PodStore) setNodeStats(stats nodeStats) {
	p.Lock()
	defer p.Unlock()
	p.nodeInfo.nodeStats = stats
}

func (p *PodStore) getNodeStats() nodeStats {
	p.Lock()
	defer p.Unlock()
	return p.nodeInfo.nodeStats
}

func (p *PodStore) refresh(now time.Time) {
	podList, _ := p.kubeClient.ListPods()
	p.refreshInternal(now, podList)
	p.nodeInfo.refreshEbsId()
}

func (p *PodStore) cleanup(now time.Time) {
	for _, prevMeasurement := range p.prevMeasurements {
		prevMeasurement.CleanUp(now)
	}
	p.nodeInfo.cleanUp(now)

	p.Lock()
	defer p.Unlock()
	p.cache.CleanUp(now)
}

func (p *PodStore) refreshInternal(now time.Time, podList []corev1.Pod) {
	var podCount int
	var containerCount int
	var cpuRequest int64
	var memRequest int64

	for _, pod := range podList {
		podKey := createPodKeyFromMetaData(&pod)
		if podKey == "" {
			log.Printf("W! podKey is unavailable refresh pod store for pod %s", pod.Name)
			continue
		}
		tmpCpuReq, _ := getResourceSettingForPod(&pod, p.nodeInfo.getCPUCapacity(), cpuKey, getRequestForContainer)
		cpuRequest += tmpCpuReq
		tmpMemReq, _ := getResourceSettingForPod(&pod, p.nodeInfo.getMemCapacity(), memoryKey, getRequestForContainer)
		memRequest += tmpMemReq
		if pod.Status.Phase == corev1.PodRunning {
			podCount += 1
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running != nil {
				containerCount += 1
			}
		}

		p.setCachedEntry(podKey, &cachedEntry{
			pod:      pod,
			creation: now})
	}

	p.setNodeStats(nodeStats{podCnt: podCount, containerCnt: containerCount, memReq: memRequest, cpuReq: cpuRequest})
}

func (p *PodStore) decorateDiskDevice(metric telegraf.Metric, tags map[string]string) {
	if tags[MetricType] == TypeContainerFS || tags[MetricType] == TypeNodeFS || tags[MetricType] == TypeNodeDiskIO || tags[MetricType] == TypeContainerDiskIO {
		if deviceName, ok := tags[DiskDev]; ok {
			if volId := p.nodeInfo.getEbsVolumeId(deviceName); volId != "" {
				metric.AddTag(EbsVolumeId, volId)
			}
		}
	}
}

func (p *PodStore) decorateNode(metric telegraf.Metric) {
	nodeStats := p.getNodeStats()

	if metric.HasField(MetricName(TypeNode, CpuTotal)) {
		metric.AddField(MetricName(TypeNode, CpuLimit), p.nodeInfo.getCPUCapacity())
		metric.AddField(MetricName(TypeNode, CpuRequest), nodeStats.cpuReq)
		if p.nodeInfo.getCPUCapacity() != 0 {
			metric.AddField(MetricName(TypeNode, CpuUtilization), metric.Fields()[MetricName(TypeNode, CpuTotal)].(float64)/float64(p.nodeInfo.getCPUCapacity())*100)
			metric.AddField(MetricName(TypeNode, CpuReservedCapacity), float64(nodeStats.cpuReq)/float64(p.nodeInfo.getCPUCapacity())*100)
		}
	}

	if metric.HasField(MetricName(TypeNode, MemWorkingset)) {
		metric.AddField(MetricName(TypeNode, MemLimit), p.nodeInfo.getMemCapacity())
		metric.AddField(MetricName(TypeNode, MemRequest), nodeStats.memReq)
		if p.nodeInfo.getMemCapacity() != 0 {
			metric.AddField(MetricName(TypeNode, MemUtilization), float64(metric.Fields()[MetricName(TypeNode, MemWorkingset)].(uint64))/float64(p.nodeInfo.getMemCapacity())*100)
			metric.AddField(MetricName(TypeNode, MemReservedCapacity), float64(nodeStats.memReq)/float64(p.nodeInfo.getMemCapacity())*100)
		}
	}

	metric.AddField(MetricName(TypeNode, RunningPodCount), nodeStats.podCnt)
	metric.AddField(MetricName(TypeNode, RunningContainerCount), nodeStats.containerCnt)
}

func (p *PodStore) decorateCpu(metric telegraf.Metric, tags map[string]string, pod *corev1.Pod) {
	if tags[MetricType] == TypePod {
		// add cpu limit and request for pod cpu
		if metric.HasField(MetricName(TypePod, CpuTotal)) {
			podCpuReq, _ := getResourceSettingForPod(pod, p.nodeInfo.getCPUCapacity(), cpuKey, getRequestForContainer)
			// set podReq to the sum of containerReq which has req
			if podCpuReq != 0 {
				metric.AddField(MetricName(TypePod, CpuRequest), podCpuReq)
			}

			if p.nodeInfo.getCPUCapacity() != 0 {
				metric.AddField(MetricName(TypePod, CpuUtilization), metric.Fields()[MetricName(TypePod, CpuTotal)].(float64)/float64(p.nodeInfo.getCPUCapacity())*100)
				if podCpuReq != 0 {
					metric.AddField(MetricName(TypePod, CpuReservedCapacity), float64(podCpuReq)/float64(p.nodeInfo.getCPUCapacity())*100)
				}
			}

			podCpuLimit, ok := getResourceSettingForPod(pod, p.nodeInfo.getCPUCapacity(), cpuKey, getLimitForContainer)
			// only set podLimit when all the containers has limit
			if ok && podCpuLimit != 0 {
				metric.AddField(MetricName(TypePod, CpuLimit), podCpuLimit)
				metric.AddField(MetricName(TypePod, CpuUtilizationOverPodLimit), metric.Fields()[MetricName(TypePod, CpuTotal)].(float64)/float64(podCpuLimit)*100)
			}
		}
	} else if tags[MetricType] == TypeContainer {
		// add cpu limit and request for container
		if metric.HasField(MetricName(TypeContainer, CpuTotal)) {
			if p.nodeInfo.getCPUCapacity() != 0 {
				metric.AddField(MetricName(TypeContainer, CpuUtilization), metric.Fields()[MetricName(TypeContainer, CpuTotal)].(float64)/float64(p.nodeInfo.getCPUCapacity())*100)
			}
			if containerName, ok := tags[ContainerNamekey]; ok {
				for _, containerSpec := range pod.Spec.Containers {
					if containerSpec.Name == containerName {
						if cpuLimit, ok := getLimitForContainer(cpuKey, containerSpec); ok {
							metric.AddField(MetricName(TypeContainer, CpuLimit), cpuLimit)
						}
						if cpuReq, ok := getRequestForContainer(cpuKey, containerSpec); ok {
							metric.AddField(MetricName(TypeContainer, CpuRequest), cpuReq)
						}
					}
				}
			}
		}
	}
}

func (p *PodStore) decorateMem(metric telegraf.Metric, tags map[string]string, pod *corev1.Pod) {
	if tags[MetricType] == TypePod {
		if metric.HasField(MetricName(TypePod, MemWorkingset)) {
			// add mem limit and request for pod mem
			podMemReq, _ := getResourceSettingForPod(pod, p.nodeInfo.getMemCapacity(), memoryKey, getRequestForContainer)
			// set podReq to the sum of containerReq which has req
			if podMemReq != 0 {
				metric.AddField(MetricName(TypePod, MemRequest), podMemReq)
			}

			if p.nodeInfo.getMemCapacity() != 0 {
				metric.AddField(MetricName(TypePod, MemUtilization), getFloat64(metric.Fields()[MetricName(TypePod, MemWorkingset)])/float64(p.nodeInfo.getMemCapacity())*100)
				if podMemReq != 0 {
					metric.AddField(MetricName(TypePod, MemReservedCapacity), float64(podMemReq)/float64(p.nodeInfo.getMemCapacity())*100)
				}
			}

			podMemLimit, ok := getResourceSettingForPod(pod, p.nodeInfo.getMemCapacity(), memoryKey, getLimitForContainer)
			// only set podLimit when all the containers has limit
			if ok && podMemLimit != 0 {
				metric.AddField(MetricName(TypePod, MemLimit), podMemLimit)
				metric.AddField(MetricName(TypePod, MemUtilizationOverPodLimit), getFloat64(metric.Fields()[MetricName(TypePod, MemWorkingset)])/float64(podMemLimit)*100)
			}
		}
	} else if tags[MetricType] == TypeContainer {
		// add mem limit and request for container
		if metric.HasField(MetricName(TypeContainer, MemWorkingset)) {
			if p.nodeInfo.getMemCapacity() != 0 {
				metric.AddField(MetricName(TypeContainer, MemUtilization), getFloat64(metric.Fields()[MetricName(TypeContainer, MemWorkingset)])/float64(p.nodeInfo.getMemCapacity())*100)
			}
			if containerName, ok := tags[ContainerNamekey]; ok {
				for _, containerSpec := range pod.Spec.Containers {
					if containerSpec.Name == containerName {
						if memLimit, ok := getLimitForContainer(memoryKey, containerSpec); ok {
							metric.AddField(MetricName(TypeContainer, MemLimit), memLimit)
						}
						if memReq, ok := getRequestForContainer(memoryKey, containerSpec); ok {
							metric.AddField(MetricName(TypeContainer, MemRequest), memReq)
						}
					}
				}
			}
		}
	}
}

func getFloat64(v interface{}) float64 {
	var value float64

	switch t := v.(type) {
	case int:
		value = float64(t)
	case int32:
		value = float64(t)
	case int64:
		value = float64(t)
	case uint:
		value = float64(t)
	case uint32:
		value = float64(t)
	case uint64:
		value = float64(t)
	case float64:
		value = t
	default:
		log.Printf("value type does not support: %v, %T", v, v)
	}
	return value
}

func (p *PodStore) addStatus(metric telegraf.Metric, tags map[string]string, pod *corev1.Pod) {
	if tags[MetricType] == TypePod {
		metric.AddField(PodStatus, string(pod.Status.Phase))
		var curContainerRestarts int
		for _, containerStatus := range pod.Status.ContainerStatuses {
			curContainerRestarts += int(containerStatus.RestartCount)
		}
		podKey := createPodKeyFromMetric(tags)
		if podKey != "" {
			content, ok := p.getPrevMeasurement(TypePod, podKey)
			if ok {
				prevMeasurement := content.(prevPodMeasurement)
				result := 0
				if curContainerRestarts > prevMeasurement.containersRestarts {
					result = curContainerRestarts - prevMeasurement.containersRestarts
				}
				metric.AddField(MetricName(TypePod, ContainerRestartCount), result)
			}
			p.setPrevMeasurement(TypePod, podKey, prevPodMeasurement{containersRestarts: curContainerRestarts})
		}
	} else if tags[MetricType] == TypeContainer {
		if containerName, ok := tags[ContainerNamekey]; ok {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.Name == containerName {
					if containerStatus.State.Running != nil {
						metric.AddField(ContainerStatus, "Running")
					} else if containerStatus.State.Waiting != nil {
						metric.AddField(ContainerStatus, "Waiting")
						if containerStatus.State.Waiting.Reason != "" {
							metric.AddField(ContainerStatusReason, containerStatus.State.Waiting.Reason)
						}
					} else if containerStatus.State.Terminated != nil {
						metric.AddField(ContainerStatus, "Terminated")
						if containerStatus.State.Terminated.Reason != "" {
							metric.AddField(ContainerStatusReason, containerStatus.State.Terminated.Reason)
						}
					}
					if containerStatus.LastTerminationState.Terminated != nil && containerStatus.LastTerminationState.Terminated.Reason != "" {
						metric.AddField(ContainerLastTerminationReason, containerStatus.LastTerminationState.Terminated.Reason)
					}
					containerKey := createContainerKeyFromMetric(tags)
					if containerKey != "" {
						content, ok := p.getPrevMeasurement(TypeContainer, containerKey)
						if ok {
							prevMeasurement := content.(prevContainerMeasurement)
							result := 0
							if int(containerStatus.RestartCount) > prevMeasurement.restarts {
								result = int(containerStatus.RestartCount) - prevMeasurement.restarts
							}
							metric.AddField(ContainerRestartCount, result)
						}
						p.setPrevMeasurement(TypeContainer, containerKey, prevContainerMeasurement{restarts: int(containerStatus.RestartCount)})
					}
				}
			}
		}
	}
}

// It could be used to get limit/request(depend on the passed-in fn) per pod
// return the sum of ResourceSetting and a bool which indicate whether all container set Resource
func getResourceSettingForPod(pod *corev1.Pod, bound int64, resource corev1.ResourceName, fn func(resource corev1.ResourceName, spec corev1.Container) (int64, bool)) (int64, bool) {
	var result int64
	allSet := true
	for _, containerSpec := range pod.Spec.Containers {
		val, ok := fn(resource, containerSpec)
		if ok {
			result += val
		} else {
			allSet = false
		}
	}
	if bound != 0 && result > bound {
		result = bound
	}
	return result, allSet
}

func getLimitForContainer(resource corev1.ResourceName, spec corev1.Container) (int64, bool) {
	if v, ok := spec.Resources.Limits[resource]; ok {
		var limit int64
		if resource == cpuKey {
			limit = v.MilliValue()
		} else {
			limit = v.Value()
		}
		return limit, true
	}
	return 0, false
}

func getRequestForContainer(resource corev1.ResourceName, spec corev1.Container) (int64, bool) {
	if v, ok := spec.Resources.Requests[resource]; ok {
		var req int64
		if resource == cpuKey {
			req = v.MilliValue()
		} else {
			req = v.Value()
		}
		return req, true
	}
	return 0, false
}

func addContainerId(pod *corev1.Pod, tags map[string]string, metric telegraf.Metric, kubernetesBlob map[string]interface{}) {
	if _, ok := tags[ContainerNamekey]; ok {
		rawId := ""
		for _, container := range pod.Status.ContainerStatuses {
			if tags[ContainerNamekey] == container.Name {
				rawId = container.ContainerID
				if rawId != "" {
					ids := strings.Split(rawId, "://")
					if len(ids) == 2 {
						kubernetesBlob[ids[0]] = map[string]string{"container_id": ids[1]}
					} else {
						log.Printf("W! Cannot parse container id from %s for container %s", rawId, container.Name)
						kubernetesBlob["container_id"] = rawId
					}
				}
				break
			}
		}
		if rawId == "" {
			kubernetesBlob["container_id"] = tags[ContainerIdkey]
		}
		metric.RemoveTag(ContainerIdkey)
	}
}

func addLabels(pod *corev1.Pod, kubernetesBlob map[string]interface{}) {
	labels := make(map[string]string)
	for k, v := range pod.Labels {
		labels[k] = v
	}
	if len(labels) > 0 {
		kubernetesBlob["labels"] = labels
	}
}

func getJobNamePrefix(podName string) string {
	return re.Split(podName, 2)[0]
}

func (p *PodStore) addPodOwnersAndPodName(metric telegraf.Metric, pod *corev1.Pod, kubernetesBlob map[string]interface{}) {
	var owners []Owner
	podName := ""
	for _, owner := range pod.OwnerReferences {
		if owner.Kind != "" && owner.Name != "" {
			kind := owner.Kind
			name := owner.Name
			if owner.Kind == ReplicaSet {
				rsToDeployment := k8sclient.Get().ReplicaSet.ReplicaSetToDeployment()
				if parent := rsToDeployment[owner.Name]; parent != "" {
					kind = Deployment
					name = parent
				} else if parent := parseDeploymentFromReplicaSet(owner.Name); parent != "" {
					profiler.Profiler.AddStats([]string{"k8sdecorator", "podstore", "rsToDeploymentMiss"}, 1)
					kind = Deployment
					name = parent
				}
			} else if owner.Kind == Job {
				if parent := parseCronJobFromJob(owner.Name); parent != "" {
					kind = CronJob
					name = parent
				} else if !p.prefFullPodName {
					name = getJobNamePrefix(name)
				}
			}
			owners = append(owners, Owner{OwnerKind: kind, OwnerName: name})

			if podName == "" {
				if owner.Kind == StatefulSet {
					podName = pod.Name
				} else if owner.Kind == DaemonSet || owner.Kind == Job || owner.Kind == ReplicaSet || owner.Kind == ReplicationController {
					podName = name
				}
			}
		}
	}
	if len(owners) > 0 {
		kubernetesBlob["pod_owners"] = owners
	}

	// if podName is not set according to a well-known controllers, then set it to its own name
	if podName == "" {
		if strings.HasPrefix(pod.Name, kubeProxy) && !p.prefFullPodName {
			podName = kubeProxy
		} else {
			podName = pod.Name
		}
	}

	metric.AddTag(PodNameKey, podName)
}

func addContainerCount(metric telegraf.Metric, tags map[string]string, pod *corev1.Pod) {
	runningContainerCount := 0
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running != nil {
			runningContainerCount += 1
		}
	}
	if tags[MetricType] == TypePod {
		metric.AddField(MetricName(TypePod, RunningContainerCount), runningContainerCount)
		metric.AddField(MetricName(TypePod, ContainerCount), len(pod.Status.ContainerStatuses))
	}
}
