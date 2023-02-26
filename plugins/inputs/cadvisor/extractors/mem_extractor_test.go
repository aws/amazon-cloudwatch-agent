// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extractors

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	cinfo "github.com/google/cadvisor/info/v1"
)

func TestMemStats(t *testing.T) {

	var result []*cinfo.ContainerInfo
	containers := map[string]*cinfo.ContainerInfo{}
	err := json.Unmarshal([]byte(PreInfo), &containers)

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
	extractor := NewMemMetricExtractor()

	extractor.preInfos.Set(result[0].Name, result[0])

	var result2 []*cinfo.ContainerInfo
	containers = map[string]*cinfo.ContainerInfo{}
	err = json.Unmarshal([]byte(CurInfo), &containers)

	if err != nil {
		log.Printf("Fail to read request body: %s", err)
	}

	for _, containerInfo := range containers {
		result2 = append(result2, containerInfo)
	}
	enc.Encode(result2)

	var cMetrics []*CAdvisorMetric
	if extractor.HasValue(result2[0]) {
		cMetrics = extractor.GetValue(result2[0], containerType)
	}
	for _, cadvisorMetric := range cMetrics {
		log.Printf("cadvisor Metrics received:\n %v \n", *cadvisorMetric)
	}
	//AssertContainsTaggedFloat(t, cMetrics[0], "container_memory_working_set", 28844032, 0)
	expectedFields := map[string]interface{}{
		"container_memory_cache":                   uint64(25645056),
		"container_memory_rss":                     uint64(221184),
		"container_memory_max_usage":               uint64(90775552),
		"container_memory_mapped_file":             uint64(0),
		"container_memory_pgfault":                 float64(1000),
		"container_memory_pgmajfault":              float64(10),
		"container_memory_hierarchical_pgmajfault": float64(10),
		"container_memory_usage":                   uint64(29728768),
		"container_memory_swap":                    uint64(0),
		"container_memory_failcnt":                 uint64(0),
		"container_memory_working_set":             uint64(28844032),
		"container_memory_hierarchical_pgfault":    float64(1000),
	}
	expectedTags := map[string]string{}
	AssertContainsTaggedField(t, cMetrics[0], expectedFields, expectedTags)

}
