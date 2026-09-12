// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// Per-core Neuron attribution on multi-runtime nodes.
//
// Invariant: when several Neuron runtimes share a node, every (core, runtime)
// reading must reach the exporter under its own identity. neuron-monitor reports
// every core from every runtime -- the owning pair real, the rest zero -- so
// runtime_tag is the only attribute separating a core's real reading from another
// runtime's zero for that same core. Stop separating them and two datapoints share
// an identity, the last write wins, and for one core that is a plausible-looking 0.
//
// These run the shipped neuron.yaml -- groupbyattrs then the promote transform,
// rendered from the embedded template rather than restated, so a config change that
// breaks the invariant fails here. TestNeuronMultiRuntimeCollapsedConfigLosesData
// runs a config that violates it, proving the assertions discriminate.

package containerinsights

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"
	"text/template"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"gopkg.in/yaml.v3"
)

const (
	neuronGroupByAttrs = "groupbyattrs/cw_k8s_ci_v0_neuron"
	neuronPromote      = "transform/cw_k8s_ci_v0_neuron_promote"

	testPod       = "neuron-burn-abcde-12345"
	testNamespace = "default"
	testContainer = "burn"
	tagA          = "burn-core0"
	tagB          = "burn-core1"
)

// renderedNeuronProcessors returns the processors block of the shipped neuron.yaml.
func renderedNeuronProcessors(t *testing.T) map[string]any {
	t.Helper()
	tmpl, err := template.New("neuron").Parse(neuronYAML)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, tmpl.Execute(&buf, templateData{
		ClusterName:        "test-cluster",
		Region:             "us-west-2",
		CollectionInterval: "30s",
		ScrapeTimeout:      "10s",
		NodeName:           "test-node",
		HostIP:             "127.0.0.1",
	}))

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(buf.Bytes(), &parsed))
	processors, ok := parsed["processors"].(map[string]any)
	require.True(t, ok, "neuron.yaml has no processors block")
	return processors
}

func processorConfig(t *testing.T, processors map[string]any, name string) map[string]any {
	t.Helper()
	raw, ok := processors[name]
	require.True(t, ok, "neuron.yaml is missing %s", name)
	cfg, ok := raw.(map[string]any)
	require.True(t, ok, "%s is not a mapping", name)
	return cfg
}

// buildChain wires groupbyattrs -> promote -> sink from the given configs.
func buildChain(t *testing.T, gbaCfg, promoteCfg map[string]any) (processor.Metrics, *consumertest.MetricsSink) {
	t.Helper()
	sink := new(consumertest.MetricsSink)

	promote := newProcessor(t, transformprocessor.NewFactory(), "transform", promoteCfg, sink)
	gba := newProcessor(t, groupbyattrsprocessor.NewFactory(), "groupbyattrs", gbaCfg, promote)
	return gba, sink
}

func newProcessor(
	t *testing.T, factory processor.Factory, typ string, cfg map[string]any, next consumer.Metrics,
) processor.Metrics {
	t.Helper()
	c := factory.CreateDefaultConfig()
	require.NoError(t, confmap.NewFromStringMap(cfg).Unmarshal(c))
	p, err := factory.CreateMetrics(
		context.Background(), processortest.NewNopSettings(component.MustNewType(typ)), c, next)
	require.NoError(t, err)
	require.NoError(t, p.Start(context.Background(), componenttestNopHost{}))
	return p
}

type componenttestNopHost struct{}

func (componenttestNopHost) GetExtensions() map[component.ID]component.Component { return nil }

// multiRuntimeMetrics is what neuron-monitor emits for 2 cores x 2 runtimes on one
// node: the owning pair real, the other zero. Pod identity is identical across all
// four -- one pod holding both cores, which occurs in practice and is the
// collapse-prone case, since pod identity alone cannot separate the datapoints.
func multiRuntimeMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	sm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("neuroncore_utilization_ratio")
	dps := m.SetEmptyGauge().DataPoints()

	add := func(core, tag string, value float64) {
		dp := dps.AppendEmpty()
		dp.SetDoubleValue(value)
		a := dp.Attributes()
		a.PutStr("neuroncore", core)
		a.PutStr("runtime_tag", tag)
		a.PutStr("k8s.pod.name", testPod)
		a.PutStr("k8s.namespace.name", testNamespace)
		a.PutStr("k8s.container.name", testContainer)
	}
	add("0", tagA, 75.1)
	add("1", tagA, 0)
	add("0", tagB, 0)
	add("1", tagB, 75.3)
	return md
}

// observed flattens the sink into "core=<n> tag=<t> value=<v>" triples, reading the
// tag from the RESOURCE (where the promote puts it) and the core from the datapoint.
func observed(t *testing.T, sink *consumertest.MetricsSink) []string {
	t.Helper()
	var out []string
	for _, md := range sink.AllMetrics() {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			tag := "<none>"
			if v, ok := rm.Resource().Attributes().Get("aws.neuron.runtime.tag"); ok {
				tag = v.Str()
			}
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				ms := rm.ScopeMetrics().At(j).Metrics()
				for k := 0; k < ms.Len(); k++ {
					dps := ms.At(k).Gauge().DataPoints()
					for d := 0; d < dps.Len(); d++ {
						dp := dps.At(d)
						core, _ := dp.Attributes().Get("neuroncore")
						out = append(out, fmt.Sprintf("core=%s tag=%s value=%.1f",
							core.Str(), tag, dp.DoubleValue()))
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func consume(t *testing.T, chain processor.Metrics, md pmetric.Metrics) {
	t.Helper()
	require.NoError(t, chain.ConsumeMetrics(context.Background(), md))
}

// TestNeuronMultiRuntimeKeepsEveryCorePerRuntime requires all four (core, runtime)
// readings to survive the shipped config with their real values.
func TestNeuronMultiRuntimeKeepsEveryCorePerRuntime(t *testing.T) {
	processors := renderedNeuronProcessors(t)
	chain, sink := buildChain(t,
		processorConfig(t, processors, neuronGroupByAttrs),
		processorConfig(t, processors, neuronPromote))
	consume(t, chain, multiRuntimeMetrics())

	assert.Equal(t, []string{
		"core=0 tag=burn-core0 value=75.1",
		"core=0 tag=burn-core1 value=0.0",
		"core=1 tag=burn-core0 value=0.0",
		"core=1 tag=burn-core1 value=75.3",
	}, observed(t, sink),
		"both busy cores must survive with their own runtime tag; a missing 75.x "+
			"reading means one runtime's datapoint was shadowed by another's zero")
}

// TestNeuronMultiRuntimeSeparatesRuntimesIntoResources pins the mechanism: one
// ResourceMetrics per runtime is what stops the datapoints colliding.
func TestNeuronMultiRuntimeSeparatesRuntimesIntoResources(t *testing.T) {
	processors := renderedNeuronProcessors(t)
	chain, sink := buildChain(t,
		processorConfig(t, processors, neuronGroupByAttrs),
		processorConfig(t, processors, neuronPromote))
	consume(t, chain, multiRuntimeMetrics())

	tags := map[string]int{}
	total := 0
	for _, md := range sink.AllMetrics() {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			v, ok := rm.Resource().Attributes().Get("aws.neuron.runtime.tag")
			require.True(t, ok, "resource is missing aws.neuron.runtime.tag")
			tags[v.Str()]++
			total++
		}
	}
	assert.Equal(t, 2, total, "expected one ResourceMetrics per runtime")
	assert.Equal(t, map[string]int{tagA: 1, tagB: 1}, tags)
}

// TestNeuronMultiRuntimePromotesPodIdentity requires pod identity to land on the
// resource and NOT remain on the datapoint. groupbyattrs moves its grouping keys
// rather than copying them, so nothing downstream needs to re-promote them.
func TestNeuronMultiRuntimePromotesPodIdentity(t *testing.T) {
	processors := renderedNeuronProcessors(t)
	chain, sink := buildChain(t,
		processorConfig(t, processors, neuronGroupByAttrs),
		processorConfig(t, processors, neuronPromote))
	consume(t, chain, multiRuntimeMetrics())

	for _, md := range sink.AllMetrics() {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			for _, kv := range []struct{ key, want string }{
				{"k8s.pod.name", testPod},
				{"k8s.namespace.name", testNamespace},
				{"k8s.container.name", testContainer},
			} {
				v, ok := rm.Resource().Attributes().Get(kv.key)
				require.True(t, ok, "resource is missing %s", kv.key)
				assert.Equal(t, kv.want, v.Str())
			}

			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				ms := rm.ScopeMetrics().At(j).Metrics()
				for k := 0; k < ms.Len(); k++ {
					dps := ms.At(k).Gauge().DataPoints()
					for d := 0; d < dps.Len(); d++ {
						attrs := dps.At(d).Attributes()
						for _, key := range []string{
							"k8s.pod.name", "k8s.namespace.name", "k8s.container.name", "runtime_tag",
						} {
							_, present := attrs.Get(key)
							assert.False(t, present,
								"%s must be moved off the datapoint, not left on it", key)
						}
					}
				}
			}
		}
	}
}

// TestNeuronMultiRuntimeCollapsedConfigLosesData is the negative control: it runs a
// config that drops the runtime dimension and asserts the loss, so the tests above
// are known to discriminate rather than merely pass. If it ever starts passing, the
// collapse has stopped reproducing and those assertions prove nothing.
//
// The collapse needs both halves. groupbyattrs omits the runtime_tag key, so the
// runtimes are not split into separate resources; the promote then runs in
// `context: datapoint` while writing resource.attributes, which are
// per-ResourceMetrics -- so the statement executes once per datapoint and the last
// write wins -- and deletes runtime_tag from the datapoint. This is the shape the
// agent shipped with.
func TestNeuronMultiRuntimeCollapsedConfigLosesData(t *testing.T) {
	collapsedGBA := map[string]any{
		"keys": []any{"k8s.pod.name", "k8s.namespace.name", "k8s.container.name"},
	}
	collapsedPromote := map[string]any{
		"error_mode": "ignore",
		"metric_statements": []any{
			map[string]any{
				"context": "datapoint",
				"statements": []any{
					`set(resource.attributes["k8s.pod.name"], attributes["k8s.pod.name"]) where attributes["k8s.pod.name"] != nil`,
					`set(resource.attributes["k8s.namespace.name"], attributes["k8s.namespace.name"]) where attributes["k8s.namespace.name"] != nil`,
					`set(resource.attributes["k8s.container.name"], attributes["k8s.container.name"]) where attributes["k8s.container.name"] != nil`,
					`set(resource.attributes["aws.neuron.runtime.tag"], attributes["runtime_tag"]) where attributes["runtime_tag"] != nil`,
					`delete_key(attributes, "k8s.pod.name") where attributes["k8s.pod.name"] != nil`,
					`delete_key(attributes, "k8s.namespace.name") where attributes["k8s.namespace.name"] != nil`,
					`delete_key(attributes, "k8s.container.name") where attributes["k8s.container.name"] != nil`,
					`delete_key(attributes, "runtime_tag") where attributes["runtime_tag"] != nil`,
				},
			},
		},
	}

	chain, sink := buildChain(t, collapsedGBA, collapsedPromote)
	consume(t, chain, multiRuntimeMetrics())
	got := observed(t, sink)

	// One resource, one surviving tag: the four datapoints keep only two identities.
	assert.Len(t, got, 4, "the pre-fix chain still emits four datapoints")
	tags := map[string]struct{}{}
	for _, md := range sink.AllMetrics() {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			if v, ok := md.ResourceMetrics().At(i).Resource().Attributes().Get("aws.neuron.runtime.tag"); ok {
				tags[v.Str()] = struct{}{}
			}
		}
	}
	assert.Len(t, tags, 1,
		"pre-fix, all runtimes collapse onto ONE resource tag; got %v", tags)

	// With one tag on the resource, (core, tag) is no longer unique: each core
	// appears twice, once with its real value and once with another runtime's zero.
	// Downstream that is a duplicate identity and one value is lost.
	perCore := map[string][]float64{}
	for _, md := range sink.AllMetrics() {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				ms := rm.ScopeMetrics().At(j).Metrics()
				for k := 0; k < ms.Len(); k++ {
					dps := ms.At(k).Gauge().DataPoints()
					for d := 0; d < dps.Len(); d++ {
						dp := dps.At(d)
						core, _ := dp.Attributes().Get("neuroncore")
						perCore[core.Str()] = append(perCore[core.Str()], dp.DoubleValue())
					}
				}
			}
		}
	}
	for core, values := range perCore {
		assert.Len(t, values, 2,
			"pre-fix, core %s carries two datapoints under one identity %v — whichever "+
				"arrives last wins, and for one core that is a legitimate-looking 0",
			core, values)
	}
}
