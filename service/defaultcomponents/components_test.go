// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package defaultcomponents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"golang.org/x/exp/maps"

	"github.com/aws/amazon-cloudwatch-agent/internal/mapstructure"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

// TestDefaultConfigsMarshal checks that the default configuration of every component
// encodes with internal/mapstructure. The agent encodes the merged OTEL configuration
// with it at start, and exits if that fails. For example, the resource_attributes of
// receiver_creator have keys of the named string type observer.EndpointType.
func TestDefaultConfigsMarshal(t *testing.T) {
	factories, err := Factories()
	require.NoError(t, err)
	// Each configuration is a field of a struct, as it is in otelcol.Config, so that the
	// encode hooks see it as a nested value and Marshal returns a map.
	type wrapper struct {
		Config any `mapstructure:"config"`
	}
	check := func(kind string, id component.Type, cfg component.Config) {
		_, err := mapstructure.Marshal(wrapper{Config: cfg})
		assert.NoError(t, err, "%s %s", kind, id)
	}
	for id, f := range factories.Receivers {
		check("receiver", id, f.CreateDefaultConfig())
	}
	for id, f := range factories.Processors {
		check("processor", id, f.CreateDefaultConfig())
	}
	for id, f := range factories.Exporters {
		check("exporter", id, f.CreateDefaultConfig())
	}
	for id, f := range factories.Extensions {
		check("extension", id, f.CreateDefaultConfig())
	}
	for id, f := range factories.Connectors {
		check("connector", id, f.CreateDefaultConfig())
	}
}

func TestComponents(t *testing.T) {
	factories, err := Factories()
	assert.NoError(t, err)
	wantReceivers := []string{
		"awscontainerinsightreceiver",
		"awscontainerinsightskueuereceiver",
		"awsecscontainermetrics",
		"awsefareceiver",
		"awsekshyperpodreceiver",
		"awsnvmereceiver",
		"awsxray",
		"collectd",
		"filelog",
		"file_log",
		"hostmetrics",
		"jaeger",
		"journald",
		"jmx",
		"kafka",
		"kubeletstats",
		"nop",
		"otlp",
		"prometheus",
		"postgresql",
		"receiver_creator",
		"statsd",
		"systemmetrics",
		"tcplog",
		"tcp_log",
		"udplog",
		"udp_log",
		"windowseventlog",
		"windows_event_log",
		"zipkin",
	}
	gotReceivers := collections.MapSlice(maps.Keys(factories.Receivers), component.Type.String)
	assert.Equal(t, len(wantReceivers), len(gotReceivers))
	for _, typeStr := range wantReceivers {
		assert.Contains(t, gotReceivers, typeStr)
	}

	wantProcessors := []string{
		"attributes",
		"attributestocontext",
		"awsapplicationsignals",
		"awsattributelimit",
		"awsdevicepodcorrelation",
		"awsentity",
		"awsneuron",
		"batch",
		"cumulativetodelta",
		"deltatocumulative",
		"deltatorate",
		"ec2tagger",
		"filter",
		"gpuattributes",
		"groupbyattrs",
		"groupbytrace",
		"k8sattributes",
		"k8s_attributes",
		"kueueattributes",
		"memory_limiter",
		"metricsgeneration",
		"metricstarttime",
		"metric_start_time",
		"metricstransform",
		"nodemetadataenricher",
		"probabilistic_sampler",
		"resource",
		"resourcedetection",
		"rollup",
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
		"awscloudwatch",
		"awscloudwatchlogs",
		"awsemf",
		"awsxray",
		"debug",
		"nop",
		"otlphttp",
		"otlp_http",
		"prometheusremotewrite",
	}
	gotExporters := collections.MapSlice(maps.Keys(factories.Exporters), component.Type.String)
	assert.Equal(t, len(wantExporters), len(gotExporters))
	for _, typeStr := range wantExporters {
		assert.Contains(t, gotExporters, typeStr)
	}

	wantConnectors := []string{
		"count",
		"forward",
		"routing",
		"signaltometrics",
		"signal_to_metrics",
		"spanmetrics",
	}
	gotConnectors := collections.MapSlice(maps.Keys(factories.Connectors), component.Type.String)
	assert.Equal(t, len(wantConnectors), len(gotConnectors))
	for _, typeStr := range wantConnectors {
		assert.Contains(t, gotConnectors, typeStr)
	}

	wantExtensions := []string{
		"agenthealth",
		"awscloudwatchlogsprovisioner",
		"awsproxy",
		"ecs_observer",
		"entitystore",
		"file_storage",
		"headers_setter",
		"health_check",
		"k8s_observer",
		"k8smetadata",
		"nodemetadatacache",
		"oidctoken",
		"pprof",
		"selftelemetry",
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
