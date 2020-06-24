package cadvisor

import (
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

	// For "container in container" case, cadvisor will provide multiple stats for same container, for example:
	// /kubepods/burstable/<pod-id>/<container-id>
	// /kubepods/burstable/<pod-id>/<container-id>/kubepods/burstable/<pod-id>/<container-id>
	// In above example, the second path is actually for the container in container, which should be ignored
	keywordCount := strings.Count(info.Name, cadvisorPathPrefix)
	if keywordCount > 1 {
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

	// For "container in container" case, cadvisor will provide multiple stats for same pod, for example:
	// /kubepods/burstable/<pod-id>
	// /kubepods/burstable/<pod-id>/<container-id>/kubepods/burstable/<pod-id>
	// In above example, the second path is actually for the container in container, which should be ignored
	keywordCount := strings.Count(info.Name, cadvisorPathPrefix)
	if keywordCount > 1 {
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
