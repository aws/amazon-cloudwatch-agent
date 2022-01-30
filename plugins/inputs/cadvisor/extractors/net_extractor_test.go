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

func TestNetStats(t *testing.T) {

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
	extractor := NewNetMetricExtractor()

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

}
