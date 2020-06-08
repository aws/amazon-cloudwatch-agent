package extractors

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	cinfo "github.com/google/cadvisor/info/v1"
)

func TestFSStats(t *testing.T) {

	var result []*cinfo.ContainerInfo
	containers := map[string]*cinfo.ContainerInfo{}
	err := json.Unmarshal([]byte(CurInfo), &containers)

	if err != nil {
		log.Printf("Fail to read request body: %s", err)
	}

	for _, containerInfo := range containers {
		result = append(result, containerInfo)
	}

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.Encode(result)
	containerType := TypeContainer
	extractor := NewFileSystemMetricExtractor()

	var cMetrics []*CAdvisorMetric
	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], containerType)
	}
	for _, cadvisorMetric := range cMetrics {
		log.Printf("cadvisor Metrics received:\n %v \n", *cadvisorMetric)
	}
	expectedFields := map[string]interface{}{
		"container_filesystem_usage":       uint64(25661440),
		"container_filesystem_capacity":    uint64(21462233088),
		"container_filesystem_available":   uint64(0),
		"container_filesystem_utilization": float64(0.11956556381986117),
	}
	expectedTags := map[string]string{
		"device": "/dev/xvda1",
		"fstype": "vfs",
	}
	AssertContainsTaggedField(t, cMetrics[0], expectedFields, expectedTags)
}
