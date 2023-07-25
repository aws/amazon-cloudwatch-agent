// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package structuredlogsadapter

import (
	"testing"
	"time"

	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
)

func TestUtils_addKubernetesInfo(t *testing.T) {
	tags := map[string]string{ContainerNamekey: "testContainer", K8sPodNameKey: "testPod", PodIdKey: "123", K8sNamespace: "testNamespace", TypeService: "testService", NodeNameKey: "testNode"}
	m := metric.New("test", tags, map[string]interface{}{}, time.Now())
	kubernetesBlob := map[string]interface{}{}
	AddKubernetesInfo(m, kubernetesBlob)
	assert.Equal(t, "", m.Tags()[ContainerNamekey])
	assert.Equal(t, "", m.Tags()[K8sPodNameKey])
	assert.Equal(t, "", m.Tags()[PodIdKey])
	assert.Equal(t, "testNamespace", m.Tags()[K8sNamespace])
	assert.Equal(t, "testService", m.Tags()[TypeService])
	assert.Equal(t, "testNode", m.Tags()[NodeNameKey])
	assert.Equal(t, "0", m.Tags()["Version"])

	expectedKubeBlob := map[string]interface{}{"container_name": "testContainer", "host": "testNode", "namespace_name": "testNamespace", "pod_id": "123", "pod_name": "testPod", "service_name": "testService"}
	assert.Equal(t, expectedKubeBlob, kubernetesBlob)
}
