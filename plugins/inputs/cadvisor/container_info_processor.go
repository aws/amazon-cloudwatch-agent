// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cadvisor

import (
	"log"
	"path"
	"strconv"
	"strings"
	"time"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"

	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/cadvisor/extractors"
	cinfo "github.com/google/cadvisor/info/v1"
)

const (
	infraContainerName = "POD"
	podNameLable       = "io.kubernetes.pod.name"
	namespaceLable     = "io.kubernetes.pod.namespace"
	podIdLable         = "io.kubernetes.pod.uid"
	containerNameLable = "io.kubernetes.container.name"
	cadvisorPathPrefix = "kubepods"
)

type podKey struct {
	podId     string
	podName   string
	namespace string
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

		if outPodKey != nil {
			podKeys["pod"+outPodKey.podId] = *outPodKey
		}
	}

	if detailMode {
		for _, cInfo := range cInfos {
			if len(cInfo.Stats) == 0 {
				continue
			}
			metrics = append(metrics, processPod(cInfo, podKeys)...)
		}
	}

	metrics = mergeMetrics(metrics)

	now := time.Now()
	for _, extractor := range MetricsExtractors {
		extractor.CleanUp(now)
	}
	return metrics
}

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
		containerName := info.Spec.Labels[containerNameLable]
		namespace := info.Spec.Labels[namespaceLable]
		podName := info.Spec.Labels[podNameLable]
		podId := info.Spec.Labels[podIdLable]
		if containerName == "" || namespace == "" || podName == "" {
			return result, pKey
		}

		pKey = &podKey{podName: podName, podId: podId, namespace: namespace}

		tags[PodIdKey] = podId
		tags[K8sPodNameKey] = podName
		tags[K8sNamespace] = namespace
		if containerName != infraContainerName {
			tags[ContainerNamekey] = containerName
			tags[ContainerIdkey] = path.Base(info.Name)
			containerType = TypeContainer
		} else {
			containerType = TypePod
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

func processPod(info *cinfo.ContainerInfo, podKeys map[string]podKey) []*extractors.CAdvisorMetric {
	var result []*extractors.CAdvisorMetric
	if isContainerInContainer(info.Name) {
		log.Printf("D! drop metric because it's nested container, name %s", info.Name)
		return result
	}

	podKey := getPodKey(info, podKeys)
	if podKey == nil {
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

func getPodKey(info *cinfo.ContainerInfo, podKeys map[string]podKey) *podKey {
	key := path.Base(info.Name)

	if v, ok := podKeys[key]; ok {
		return &v
	}

	return nil
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
