// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package defaultcomponents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"golang.org/x/exp/maps"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

func TestComponents(t *testing.T) {
	factories, err := Factories()
	assert.NoError(t, err)
	wantReceivers := []string{
		"awscontainerinsightreceiver",
		"awscontainerinsightskueuereceiver",
		"awsecscontainermetrics",
		"awsnvmereceiver",
		"awsxray",
		"filelog",
		"jaeger",
		"jmx",
		"kafka",
		"nop",
		"otlp",
		"prometheus",
		"statsd",
		"tcplog",
		"udplog",
		"zipkin",
	}
	gotReceivers := collections.MapSlice(maps.Keys(factories.Receivers), component.Type.String)
	assert.Equal(t, len(wantReceivers), len(gotReceivers))
	for _, typeStr := range wantReceivers {
		assert.Contains(t, gotReceivers, typeStr)
	}

	wantProcessors := []string{
		"awsapplicationsignals",
		"awsentity",
		"attributes",
		"batch",
		"cumulativetodelta",
		"deltatocumulative",
		"deltatorate",
		"ec2tagger",
		"metricsgeneration",
		"filter",
		"gpuattributes",
		"kueueattributes",
		"groupbytrace",
		"groupbyattrs",
		"k8sattributes",
		"memory_limiter",
		"metricstransform",
		"resourcedetection",
		"resource",
		"rollup",
		"probabilistic_sampler",
		"span",
		"tail_sampling",
		"transform",
	}
	gotProcessors := collections.MapSlice(maps.Keys(factories.Processors), component.Type.String)
	assert.Equal(t, len(wantProcessors), len(gotProcessors))
	for _, typeStr := range wantProcessors {
		assert.Contains(t, gotProcessors, typeStr)
	}

	wantExporters := []string{
		"awscloudwatchlogs",
		"awsemf",
		"awscloudwatch",
		"awsxray",
		"debug",
		"nop",
		"prometheusremotewrite",
	}
	gotExporters := collections.MapSlice(maps.Keys(factories.Exporters), component.Type.String)
	assert.Equal(t, len(wantExporters), len(gotExporters))
	for _, typeStr := range wantExporters {
		assert.Contains(t, gotExporters, typeStr)
	}

	wantExtensions := []string{
		"agenthealth",
		"awsproxy",
		"ecs_observer",
		"entitystore",
		"k8smetadata",
		"file_storage",
		"health_check",
		"pprof",
		"server",
		"sigv4auth",
		"zpages",
	}
	gotExtensions := collections.MapSlice(maps.Keys(factories.Extensions), component.Type.String)
	assert.Equal(t, len(wantExtensions), len(gotExtensions))
	for _, typeStr := range wantExtensions {
		assert.Contains(t, gotExtensions, typeStr)
	}
}
