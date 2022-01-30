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

func TestDiskIOStats(t *testing.T) {

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
	containerType := TypeNode
	extractor := NewDiskIOMetricExtractor()

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
	expectedFieldsService := map[string]interface{}{
		"node_diskio_io_service_bytes_write": float64(10000),
		"node_diskio_io_service_bytes_total": float64(10010),
		"node_diskio_io_service_bytes_async": float64(10000),
		"node_diskio_io_service_bytes_sync":  float64(10000),
		"node_diskio_io_service_bytes_read":  float64(10),
	}
	expectedFieldsServiced := map[string]interface{}{
		"node_diskio_io_serviced_async": float64(10),
		"node_diskio_io_serviced_sync":  float64(10),
		"node_diskio_io_serviced_read":  float64(10),
		"node_diskio_io_serviced_write": float64(10),
		"node_diskio_io_serviced_total": float64(20),
	}
	expectedTags := map[string]string{
		"device": "/dev/xvda",
	}
	AssertContainsTaggedField(t, cMetrics[0], expectedFieldsService, expectedTags)
	AssertContainsTaggedField(t, cMetrics[1], expectedFieldsServiced, expectedTags)

}
