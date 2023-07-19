// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cadvisor

import (
	"log"
	"path"
	"strconv"
	"strings"
	"time"

	cinfo "github.com/google/cadvisor/info/v1"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/cadvisor/extractors"
)

const (
	// TODO: https://github.com/containerd/cri/issues/922#issuecomment-423729537 the container name can be empty on containerd
	infraContainerName = "POD"
	podNameLable       = "io.kubernetes.pod.name"
	namespaceLable     = "io.kubernetes.pod.namespace"
	podIdLable         = "io.kubernetes.pod.uid"
	containerNameLable = "io.kubernetes.container.name"
)

// podKey contains information of a pod extracted from containers owned by it.
// containers has label information for a pod and their cgroup path is one level deeper.
type podKey struct {
	cgroupPath string
	podId      string
	podName    string
	namespace  string
}

var MetricsExtractors = []extractors.MetricExtractor{
	extractors.NewCpuMetricExtractor(),
	extractors.NewMemMetricExtractor(),
	extractors.NewDiskIOMetricExtractor(),
	extractors.NewNetMetricExtractor(),
	extractors.NewFileSystemMetricExtractor(),
}

func processContainers(cInfos []*cinfo.ContainerInfo, detailMode bool, containerOrchestrator string) []*extractors.CAdvisorMetric {
	var metrics []*extractors.CAdvisorMetric
	podKeys := make(map[string]podKey)

	for _, cInfo := range cInfos {
		if len(cInfo.Stats) == 0 {
			continue
		}
		outMetrics, outPodKey := processContainer(cInfo, detailMode, containerOrchestrator)
		metrics = append(metrics, outMetrics...)
		// Save pod cgroup path we collected from containers under it.
		if outPodKey != nil {
			podKeys[outPodKey.cgroupPath] = *outPodKey
		}
	}

	beforePod := len(metrics)
	if detailMode {
		for _, cInfo := range cInfos {
			if len(cInfo.Stats) == 0 {
				continue
			}
			metrics = append(metrics, processPod(cInfo, podKeys)...)
		}
	}
	// This happens when our cgroup path and label based pod detection logic is not working.
	// contained https://github.com/aws/amazon-cloudwatch-agent/issues/188
	// docker systemd https://github.com/aws/amazon-cloudwatch-agent/pull/171
	if len(metrics) == beforePod {
		log.Printf("W! No pod metric collected, metrics count is still %d is containerd socket mounted? https://github.com/aws/amazon-cloudwatch-agent/issues/188", beforePod)
	}

	metrics = mergeMetrics(metrics)

	now := time.Now()
	for _, extractor := range MetricsExtractors {
		extractor.CleanUp(now)
	}
	return metrics
}

// processContainers get metrics for individual container and gather information for pod so we can look it up later.
func processContainer(info *cinfo.ContainerInfo, detailMode bool, containerOrchestrator string) ([]*extractors.CAdvisorMetric, *podKey) {
	var result []*extractors.CAdvisorMetric
	var pKey *podKey

	if isContainerInContainer(info.Name) {
		log.Printf("D! drop metric because it's nested container, name %s", info.Name)
		return result, pKey
	}

	tags := map[string]string{}

	var containerType string
	if info.Name != "/" {
		if !detailMode {
			return result, pKey
		}

		// Only a container has all these three labels set.
		containerName := info.Spec.Labels[containerNameLable]
		namespace := info.Spec.Labels[namespaceLable]
		podName := info.Spec.Labels[podNameLable]
		podId := info.Spec.Labels[podIdLable]
		// NOTE: containerName can be empty for pause container on containerd
		// https://github.com/containerd/cri/issues/922#issuecomment-423729537
		if namespace == "" || podName == "" {
			return result, pKey
		}

		// Pod's cgroup path is parent for a container.
		// container name: /kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod04d39715_075e_4c7c_b128_67f7897c05b7.slice/docker-57b3dabd69b94beb462244a0c15c244b509adad0940cdcc67ca079b8208ec1f2.scope
		// pod name:       /kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod04d39715_075e_4c7c_b128_67f7897c05b7.slice/
		podPath := path.Dir(info.Name)
		pKey = &podKey{cgroupPath: podPath, podName: podName, podId: podId, namespace: namespace}

		tags[PodIdKey] = podId
		tags[K8sPodNameKey] = podName
		tags[K8sNamespace] = namespace

		switch containerName {
		// For docker, pause container name is set to POD while containerd does not set it.
		// See https://github.com/aws/amazon-cloudwatch-agent/issues/188
		case "", infraContainerName:
			// NOTE: the pod here is only used by NetMetricExtractor,
			// other pod info like CPU, Mem are dealt within in processPod.
			containerType = TypeInfraContainer
		default:
			tags[ContainerNamekey] = containerName
			tags[ContainerIdkey] = path.Base(info.Name)
			containerType = TypeContainer

			// TODO(pingleig): wait for upstream fix https://github.com/aws/amazon-cloudwatch-agent/issues/192
			if !info.Spec.HasFilesystem {
				log.Printf("D! containerd does not have container filesystem metrics from cadvisor, See https://github.com/aws/amazon-cloudwatch-agent/issues/192")
			}
		}
	} else {
		containerType = TypeNode
		if containerOrchestrator == ECS {
			containerType = TypeInstance
		}
	}

	tags[Timestamp] = strconv.FormatInt(extractors.GetStats(info).Timestamp.UnixNano()/1000000, 10)

	for _, extractor := range MetricsExtractors {
		if extractor.HasValue(info) {
			result = append(result, extractor.GetValue(info, containerType)...)
		}
	}

	for _, ele := range result {
		ele.AddTags(tags)
	}
	return result, pKey
}

// processPod is almost identical as processContainer. We got this second loop because pod detection relies
// on inspecting labels from containers in processContainer. cgroup path for detected pods are saved in podKeys.
// We may not get container before pod when looping all returned cgroup paths so we use a two pass solution
// in processContainers.
func processPod(info *cinfo.ContainerInfo, podKeys map[string]podKey) []*extractors.CAdvisorMetric {
	var result []*extractors.CAdvisorMetric
	if isContainerInContainer(info.Name) {
		log.Printf("D! drop metric because it's nested container, name %s", info.Name)
		return result
	}

	podKey, ok := podKeys[info.Name]
	if !ok {
		return result
	}

	tags := map[string]string{}
	tags[PodIdKey] = podKey.podId
	tags[K8sPodNameKey] = podKey.podName
	tags[K8sNamespace] = podKey.namespace

	tags[Timestamp] = strconv.FormatInt(extractors.GetStats(info).Timestamp.UnixNano()/1000000, 10)

	for _, extractor := range MetricsExtractors {
		if extractor.HasValue(info) {
			result = append(result, extractor.GetValue(info, TypePod)...)
		}
	}

	for _, ele := range result {
		ele.AddTags(tags)
	}
	return result
}

// Check if it's a container running inside container, caller will drop the metric when return value is true.
// The validation is based on ContainerReference.Name, which is essentially cgroup path.
// The first version is from https://github.com/aws/amazon-cloudwatch-agent/commit/e8daa5f5926c5a5f38e0ceb746c141be463e11e4#diff-599185154c116b295172b56311729990d20672f6659500870997c018ce072100
// But the logic no longer works when docker is using systemd as cgroup driver, because a prefix like `kubepods` is attached to each segment.
// The new name pattern with systemd is
// - Guaranteed /kubepods.slice/kubepods-podc8f7bb69_65f2_4b61_ae5a_9b19ac47a239.slice/docker-523b624a86a2a74c2bedf586d8448c86887ef7858a8dec037d6559e5ad3fccb5.scope
// - Burstable /kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-podab0e310c_0bdb_48e8_ac87_81a701514645.slice/docker-caa8a5e51cd6610f8f0110b491e8187d23488b9635acccf0355a7975fd3ff158.scope
// - Docker in Docker /kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podc9adcee4_c874_4dad_8bc8_accdbd67ac3a.slice/docker-e58cfbc8b67f6e1af458efdd31cb2a8abdbf9f95db64f4c852b701285a09d40e.scope/docker/fb651068cfbd4bf3d45fb092ec9451f8d1a36b3753687bbaa0a9920617eae5b9
// So we check the number of segements within the cgroup path to determine if it's a container running in container.
func isContainerInContainer(p string) bool {
	segs := strings.Split(strings.TrimLeft(p, "/"), "/")
	// Without nested container, the number of segments (regardless of cgroupfs/systemd) are either 3 or 4 (depends on QoS)
	// /kubepods/pod_id/docker_id
	// /kubepods/qos/pod_id/docker_id
	// With nested container, the number of segments are either 5 or 6
	// /kubepods/pod_id/docker_id/docker/docker_id
	// /kubepods/qos/pod_id/docker_id/docker/docker_id
	return len(segs) > 4
}
