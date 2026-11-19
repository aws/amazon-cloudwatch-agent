// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"gopkg.in/yaml.v3"
)

// loadVLLMRuleFile returns the connector configuration in
// testdata/vllm_rule.yaml, validated.
func loadVLLMRuleFile(t testing.TB) *Config {
	t.Helper()
	conf, err := confmaptest.LoadConf(filepath.Join("testdata", "vllm_rule.yaml"))
	require.NoError(t, err)
	cfg := createDefaultConfig().(*Config)
	require.NoError(t, conf.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())
	return cfg
}

// TestVLLMRuleFile pins the expressions of testdata/vllm_rule.yaml, which are
// the ones the Helm chart renders, so that YAML quoting cannot silently
// change a pattern.
func TestVLLMRuleFile(t *testing.T) {
	cfg := loadVLLMRuleFile(t)
	require.Len(t, cfg.Rules, 1)
	rule := cfg.Rules[0]
	assert.Equal(t, "vllm_engine_config_info", rule.Name)
	assert.Equal(t, "vLLM engine configuration parsed from the engine startup log line. The value is always 1.", rule.Description)
	assert.Equal(t, 500, rule.MaxEntities)
	assert.Equal(t, []string{"Initializing an LLM engine (v", "Initializing a V0 LLM engine (v", "Initializing a V1 LLM engine (v"}, rule.BodyContains)
	assert.Empty(t, rule.Conditions)
	assert.Equal(t, []string{"k8s.namespace.name", "k8s.pod.name", "k8s.container.name"}, rule.Identity)
	assert.Equal(t, []string{"k8s.namespace.name", "k8s.pod.name", "k8s.container.name", "k8s.pod.uid"}, rule.IncludeResourceAttributes)

	extract := func(pattern string) string {
		return `ExtractPatterns(log.body, "` + pattern + `")["v"]`
	}
	optional := func(key, pattern string) AttributeConfig {
		return AttributeConfig{Key: key, Value: extract(pattern), Optional: true}
	}
	assert.Equal(t, []AttributeConfig{
		{Key: "vllm_version", Value: extract(`Initializing an? (?:V[01] )?LLM engine \\(v(?P<v>[^)]+)\\) with config: `)},
		{Key: "model", Value: extract(`with config: model='(?P<v>[^']*)'`)},
		{Key: "vllm_engine", Value: extract(`Initializing a (?P<v>V[01]) LLM engine`), DefaultValue: "V0"},
		optional("served_model_name", `, served_model_name=(?P<v>[^,]+),`),
		optional("dtype", `, dtype=(?:torch\\.)?(?P<v>[A-Za-z0-9_]+),`),
		optional("max_seq_len", `, max_seq_len=(?P<v>\\d+),`),
		optional("tensor_parallel_size", `, tensor_parallel_size=(?P<v>\\d+),`),
		optional("pipeline_parallel_size", `, pipeline_parallel_size=(?P<v>\\d+),`),
		optional("data_parallel_size", `, data_parallel_size=(?P<v>\\d+),`),
		optional("quantization", `, quantization=(?P<v>[A-Za-z0-9_.\\-]+),`),
		optional("enforce_eager", `, enforce_eager=(?P<v>True|False),`),
		optional("kv_cache_dtype", `, kv_cache_dtype=(?P<v>[A-Za-z0-9_]+),`),
		optional("enable_prefix_caching", `, enable_prefix_caching=(?P<v>True|False|None),`),
		optional("speculative_method", `speculative_config=(?:SpeculativeConfig\\(method=')?(?P<v>None|[^',)]+)`),
	}, rule.Attributes)
}

// TestREADMEExample checks that the example in README.md is a valid
// configuration and that its rule is the one in testdata/vllm_rule.yaml.
func TestREADMEExample(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	require.NoError(t, err)
	// Git for Windows checks out text files with CRLF line endings by default.
	text := strings.ReplaceAll(string(readme), "\r\n", "\n")
	_, after, ok := strings.Cut(text, "\n## Example\n")
	require.True(t, ok, "README.md has an Example section")
	_, after, ok = strings.Cut(after, "```yaml\n")
	require.True(t, ok, "the Example section has a YAML block")
	example, _, ok := strings.Cut(after, "```")
	require.True(t, ok, "the YAML block is closed")

	var raw map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(example), &raw))
	sub, err := confmap.NewFromStringMap(raw).Sub("connectors::" + TypeStr + "/vllm")
	require.NoError(t, err)
	cfg := createDefaultConfig().(*Config)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())

	require.NotNil(t, cfg.Storage)
	assert.Equal(t, "file_storage/state", cfg.Storage.String())
	assert.Equal(t, 30*time.Second, cfg.EmitInterval)
	assert.Equal(t, loadVLLMRuleFile(t).Rules, cfg.Rules)
}

// fixture is one file of testdata/vllm, a container log in CRI format.
type fixture struct {
	pod, namespace, container string
	lines                     []criLine
}

type criLine struct {
	ts   time.Time
	body string
}

// loadFixtures reads testdata/vllm/<pod>_<namespace>_<container>-<id>.log,
// sorted by file name.
func loadFixtures(t testing.TB) []fixture {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "vllm", "*.log"))
	require.NoError(t, err)
	require.Len(t, paths, 5)
	slices.Sort(paths)
	fixtures := make([]fixture, 0, len(paths))
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".log")
		parts := strings.SplitN(name, "_", 3)
		require.Len(t, parts, 3, path)
		dash := strings.LastIndexByte(parts[2], '-')
		require.Positive(t, dash, path)
		f := fixture{pod: parts[0], namespace: parts[1], container: parts[2][:dash]}

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
			f.lines = append(f.lines, parseCRI(t, line))
		}
		fixtures = append(fixtures, f)
	}
	return fixtures
}

// parseCRI splits a CRI log line, "<timestamp> <stream> <tag> <message>",
// and returns the timestamp and the message, which is the record body.
func parseCRI(t testing.TB, line string) criLine {
	t.Helper()
	parts := strings.SplitN(line, " ", 4)
	require.Len(t, parts, 4, line)
	require.Equal(t, "F", parts[2], "only full lines are expected: %s", line)
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	require.NoError(t, err, line)
	return criLine{ts: ts, body: parts[3]}
}

// resource returns the resource attributes of a fixture's container: the
// names in the log file name, and a synthetic pod UID.
func (f *fixture) resource() map[string]string {
	return map[string]string{
		"k8s.pod.name":       f.pod,
		"k8s.namespace.name": f.namespace,
		"k8s.container.name": f.container,
		"k8s.pod.uid":        "uid-" + f.pod,
	}
}

// vllmPodResource returns the resource attributes of a record of the vllm
// container of a pod in the inference namespace: the names that the filelog
// receiver reads from the log file name, and the pod UID that k8sattributes
// adds.
func vllmPodResource(pod string) map[string]string {
	return map[string]string{
		"k8s.namespace.name": "inference",
		"k8s.pod.name":       pod,
		"k8s.container.name": "vllm",
		"k8s.pod.uid":        "uid-" + pod,
	}
}

// vllmKey returns the state key of the vllm container of a pod in the
// inference namespace.
func vllmKey(pod string) string {
	return encodeIdentity([]kv{{"k8s.namespace.name", "inference"}, {"k8s.pod.name", pod}, {"k8s.container.name", "vllm"}})
}

// fixtureLogs returns all fixtures as one batch, one ResourceLogs per file.
func fixtureLogs(fixtures []fixture) plog.Logs {
	ld := plog.NewLogs()
	for i := range fixtures {
		lrs := addResource(ld, fixtures[i].resource())
		for _, line := range fixtures[i].lines {
			addRecord(lrs, line.ts, line.body)
		}
	}
	return ld
}

// orderedPairs reads a YAML mapping in document order, with every value
// verbatim as a string.
type orderedPairs []kv

func (p *orderedPairs) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: expected a mapping", node.Line)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k, v := node.Content[i], node.Content[i+1]
		if k.Kind != yaml.ScalarNode || v.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: expected scalar keys and values", k.Line)
		}
		*p = append(*p, kv{Key: k.Value, Value: v.Value})
	}
	return nil
}

// goldenSeries is one expected series in testdata/vllm_golden.yaml.
type goldenSeries struct {
	Resource       orderedPairs `yaml:"resource"`
	StartTimestamp string       `yaml:"start_timestamp"`
	Attributes     orderedPairs `yaml:"attributes"`
}

func loadGolden(t *testing.T) []goldenSeries {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "vllm_golden.yaml"))
	require.NoError(t, err)
	var golden []goldenSeries
	require.NoError(t, yaml.Unmarshal(data, &golden))
	require.Len(t, golden, 5)
	return golden
}

// toGolden converts emitted series to the golden form, sorted by pod name,
// after checking the fields that are the same for every series.
func toGolden(t *testing.T, got []series) []goldenSeries {
	t.Helper()
	out := make([]goldenSeries, 0, len(got))
	for _, s := range got {
		assert.Equal(t, "vllm_engine_config_info", s.name)
		assert.Equal(t, "vLLM engine configuration parsed from the engine startup log line. The value is always 1.", s.description)
		assert.Empty(t, s.unit)
		assert.Equal(t, scopeName, s.scopeName)
		assert.Equal(t, int64(1), s.value)
		assert.Equal(t, testNow, s.ts)
		out = append(out, goldenSeries{
			Resource:       orderedPairs(s.resource),
			StartTimestamp: s.start.UTC().Format(time.RFC3339Nano),
			Attributes:     orderedPairs(s.attrs),
		})
	}
	slices.SortFunc(out, func(a, b goldenSeries) int {
		return strings.Compare(goldenPod(a), goldenPod(b))
	})
	return out
}

// goldenPod returns the pod name of a series.
func goldenPod(s goldenSeries) string {
	return pairValue(s.Resource, "k8s.pod.name")
}

// assertGolden compares series one pod at a time, for readable diffs.
func assertGolden(t *testing.T, want, got []goldenSeries) {
	t.Helper()
	require.Len(t, got, len(want))
	for i := range want {
		assert.Equal(t, want[i], got[i], "series %d (%s)", i, goldenPod(want[i]))
	}
}

// TestVLLMGolden runs the container logs in testdata/vllm through the vLLM
// rule and compares the emitted series with testdata/vllm_golden.yaml.
func TestVLLMGolden(t *testing.T) {
	golden := loadGolden(t)
	fixtures := loadFixtures(t)

	t.Run("one_batch", func(t *testing.T) {
		sink := new(consumertest.MetricsSink)
		conn, _ := newTestConnector(t, loadVLLMRuleFile(t), sink)
		ld := fixtureLogs(fixtures)
		ld.MarkReadOnly()
		require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
		require.Len(t, sink.AllMetrics(), 1)
		assertGolden(t, golden, toGolden(t, allSeries(t, sink.AllMetrics())))

		// Replaying the same logs changes nothing, so nothing is emitted.
		require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
		assert.Len(t, sink.AllMetrics(), 1)
	})

	t.Run("one_record_per_call", func(t *testing.T) {
		sink := new(consumertest.MetricsSink)
		conn, _ := newTestConnector(t, loadVLLMRuleFile(t), sink)
		calls := 0
		for i := range fixtures {
			for _, line := range fixtures[i].lines {
				ld := plog.NewLogs()
				addRecord(addResource(ld, fixtures[i].resource()), line.ts, line.body)
				require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
				calls++
			}
		}
		require.Equal(t, 11, calls)
		// Only the five startup lines emit.
		require.Len(t, sink.AllMetrics(), 5)
		assertGolden(t, golden, toGolden(t, allSeries(t, sink.AllMetrics())))
	})
}

// TestVLLMNonStartupLines checks that the other fixture lines, including the
// decoy that mentions "LLM engine (v1)", produce nothing on their own.
func TestVLLMNonStartupLines(t *testing.T) {
	var lines []string
	for _, f := range loadFixtures(t) {
		for _, line := range f.lines {
			if !strings.Contains(line.body, " with config: ") {
				lines = append(lines, line.body)
			}
		}
	}
	require.Len(t, lines, 6)
	require.True(t, slices.ContainsFunc(lines, func(s string) bool {
		return strings.Contains(s, "This text mentions the LLM engine (v1) but is not a startup line")
	}), "the decoy line is in the fixtures")
	lines = append(lines,
		// The phrase without the config: vllm_version is required.
		"WARNING 10-19 12:00:00 [core.py:90] Initializing a V1 LLM engine (v0.11.0) failed, retrying",
		// No model: model is required.
		"Initializing a V1 LLM engine (v0.11.0) with config: speculative_config=None, seed=0",
	)
	logs := func(body string) plog.Logs {
		ld := plog.NewLogs()
		addRecord(addResource(ld, vllmPodResource("vllm-0")), base, body)
		return ld
	}
	for _, body := range lines {
		sink := new(consumertest.MetricsSink)
		conn, _ := newTestConnector(t, loadVLLMRuleFile(t), sink)
		require.NoError(t, conn.ConsumeLogs(t.Context(), logs(body)))
		assert.Empty(t, sink.AllMetrics(), body)
		assert.Empty(t, conn.state.snapshot().Rules, body)
	}

	// The same resource with a startup line gives a series.
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, loadVLLMRuleFile(t), sink)
	require.NoError(t, conn.ConsumeLogs(t.Context(), logs(vllmStartupLine)))
	assert.Len(t, allSeries(t, sink.AllMetrics()), 1)
}

// TestVLLMTwoContainersInOnePod checks that two containers of one pod that
// both log a startup line give two series. This is why the container name
// is in the identity. With only the namespace and the pod name, the newer
// startup line replaces the facts of the other container. The pod UID is
// not in the identity, so the records give two series with and without it.
// k8sattributes adds the UID only when it knows the pod, and a record
// without the UID gives a series whose resource has no UID.
func TestVLLMTwoContainersInOnePod(t *testing.T) {
	const (
		prefill = "INFO 10-15 12:00:00 [core.py:77] Initializing a V1 LLM engine (v0.11.0) with config: model='m', served_model_name=prefill, tensor_parallel_size=2, seed=0"
		decode  = "INFO 10-15 12:00:01 [core.py:77] Initializing a V1 LLM engine (v0.11.0) with config: model='m', served_model_name=decode, tensor_parallel_size=4, seed=0"
	)
	resource := func(container string) []kv {
		return []kv{{"k8s.namespace.name", "inference"}, {"k8s.pod.name", "vllm-pd-0"}, {"k8s.container.name", container}}
	}
	// withUID is the resource of a series of a record with the pod UID: the
	// UID comes last, as in include_resource_attributes.
	withUID := func(container string) []kv {
		return append(resource(container), kv{"k8s.pod.uid", "uid-vllm-pd-0"})
	}
	facts := func(name, tp string) []kv {
		return []kv{{"vllm_version", "0.11.0"}, {"model", "m"}, {"vllm_engine", "V1"}, {"served_model_name", name}, {"tensor_parallel_size", tp}}
	}
	for _, tc := range []struct {
		name     string
		identity []string
		podUID   bool
		want     [][2][]kv
	}{
		{
			name:   "with_pod_uid",
			podUID: true,
			want:   [][2][]kv{{withUID("decode"), facts("decode", "4")}, {withUID("prefill"), facts("prefill", "2")}},
		},
		{
			name: "without_pod_uid",
			want: [][2][]kv{{resource("decode"), facts("decode", "4")}, {resource("prefill"), facts("prefill", "2")}},
		},
		{
			name:     "without_container_in_identity",
			identity: []string{"k8s.namespace.name", "k8s.pod.name"},
			podUID:   true,
			want:     [][2][]kv{{withUID("decode"), facts("decode", "4")}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := loadVLLMRuleFile(t)
			if tc.identity != nil {
				cfg.Rules[0].Identity = tc.identity
			}
			sink := new(consumertest.MetricsSink)
			conn, _ := newTestConnector(t, cfg, sink)
			ld := plog.NewLogs()
			for i, c := range []struct{ container, body string }{{"prefill", prefill}, {"decode", decode}} {
				attrs := map[string]string{}
				for _, p := range resource(c.container) {
					attrs[p.Key] = p.Value
				}
				if tc.podUID {
					attrs["k8s.pod.uid"] = "uid-vllm-pd-0"
				}
				addRecord(addResource(ld, attrs), base.Add(time.Duration(i)*time.Second), c.body)
			}
			require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

			var got [][2][]kv
			for _, s := range allSeries(t, sink.AllMetrics()) {
				got = append(got, [2][]kv{s.resource, s.attrs})
			}
			slices.SortFunc(got, func(a, b [2][]kv) int {
				return strings.Compare(pairValue(a[0], "k8s.container.name"), pairValue(b[0], "k8s.container.name"))
			})
			assert.Equal(t, tc.want, got)
			assert.Len(t, conn.state.snapshot().Rules["vllm_engine_config_info"], len(tc.want))
		})
	}
}

// TestVLLMReplacedPod checks that a new pod with the name of a previous pod
// replaces the facts of the previous pod. A StatefulSet, for example, gives
// the new pod the same name, and the new pod has a new UID. The UID is not
// in the identity, so the two pods are one entity. The UID is an included
// resource attribute, so the new UID is a change, also with the same startup
// line. The connector starts a new series and emits it on change, and the
// pipeline can drop the series of the previous pod.
func TestVLLMReplacedPod(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, loadVLLMRuleFile(t), sink)
	logs := func(uid string, ts time.Time) plog.Logs {
		attrs := vllmPodResource("vllm-0")
		attrs["k8s.pod.uid"] = uid
		ld := plog.NewLogs()
		addRecord(addResource(ld, attrs), ts, vllmStartupLine)
		return ld
	}
	resource := func(uid string) []kv {
		return []kv{{"k8s.namespace.name", "inference"}, {"k8s.pod.name", "vllm-0"}, {"k8s.container.name", "vllm"}, {"k8s.pod.uid", uid}}
	}
	facts := []kv{{"vllm_version", "0.11.0"}, {"model", "m"}, {"vllm_engine", "V1"}, {"tensor_parallel_size", "1"}}

	require.NoError(t, conn.ConsumeLogs(t.Context(), logs("uid-old", base)))
	require.Len(t, sink.AllMetrics(), 1)
	first := allSeries(t, sink.AllMetrics())
	require.Len(t, first, 1)
	assert.Equal(t, resource("uid-old"), first[0].resource)
	assert.Equal(t, base, first[0].start)

	// The new pod logs the same startup line a minute later.
	replaced := base.Add(time.Minute)
	require.NoError(t, conn.ConsumeLogs(t.Context(), logs("uid-new", replaced)))
	require.Len(t, sink.AllMetrics(), 2, "the new UID is a change, so the connector emits on change")
	got := allSeries(t, sink.AllMetrics()[1:])
	require.Len(t, got, 1)
	assert.Equal(t, resource("uid-new"), got[0].resource)
	assert.Equal(t, facts, got[0].attrs)
	assert.Equal(t, replaced, got[0].start, "the new series starts at the startup line of the new pod")

	entities := conn.state.snapshot().Rules["vllm_engine_config_info"]
	require.Len(t, entities, 1, "the two pods are one entity")
	e, ok := entities[vllmKey("vllm-0")]
	require.True(t, ok)
	assert.Equal(t, resource("uid-new"), e.Resource)
	assert.Equal(t, facts, e.Facts)
	assert.Equal(t, replaced.UnixNano(), e.FirstSeen, "FirstSeen is reset")
	assert.Equal(t, replaced.UnixNano(), e.Updated)
}

// TestVLLMInvalidUTF8 sends a startup line with an invalid byte in the model
// name, as a receiver without UTF-8 validation passes it. The model
// label has U+FFFD in place of the byte, and the metrics marshal to OTLP
// protobuf.
func TestVLLMInvalidUTF8(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, loadVLLMRuleFile(t), sink)
	line := strings.Replace(vllmStartupLine, "model='m'", "model='org/m\xffdel'", 1)
	require.NotEqual(t, vllmStartupLine, line)
	ld := plog.NewLogs()
	addRecord(addResource(ld, vllmPodResource("vllm-0")), base, line)
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

	got := allSeries(t, sink.AllMetrics())
	require.Len(t, got, 1)
	assert.Equal(t, []kv{{"vllm_version", "0.11.0"}, {"model", "org/m\uFFFDdel"}, {"vllm_engine", "V1"}, {"tensor_parallel_size", "1"}}, got[0].attrs)
	assertValidOTLP(t, sink.AllMetrics()[0])
}

// noMatchBodies are typical vLLM and application lines that the vLLM rule
// does not match.
var noMatchBodies = []string{
	`(APIServer pid=1) INFO:     10.0.3.7:51234 - "POST /v1/chat/completions HTTP/1.1" 200 OK`,
	`(APIServer pid=1) INFO 10-15 12:43:00 [loggers.py:259] Engine 000: Avg prompt throughput: 512.0 tokens/s, Avg generation throughput: 88.1 tokens/s, Running: 4 reqs, Waiting: 0 reqs, GPU KV cache usage: 12.3%, Prefix cache hit rate: 45.6%`,
	`(APIServer pid=1) INFO 10-15 12:41:40 [api_utils.py:357] non-default args: {'model': 'x', 'tensor_parallel_size': 4}`,
	`(APIServer pid=1) INFO 10-15 12:43:10 [doc.py:1] This text mentions the LLM engine (v1) but is not a startup line`,
	`(EngineCore pid=211) INFO 10-15 12:41:50 [gpu_model_runner.py:2000] Model loading took 65.3 GiB and 41.2 seconds`,
	`(EngineCore pid=211) INFO 10-15 12:42:10 [kv_cache_utils.py:900] GPU KV cache size: 1,234,567 tokens`,
	`{"level":"info","ts":"2026-10-15T12:40:00.123Z","caller":"server/handler.go:88","msg":"request handled","path":"/healthz","status":200,"latency_ms":3}`,
	`2026-10-15 12:40:00,123 INFO [main] com.example.OrderService - Processed order 12345 in 23 ms`,
	`10.0.3.9 - - [15/Oct/2026:12:40:00 +0000] "GET /metrics HTTP/1.1" 200 5123 "-" "Prometheus/2.53.0"`,
	`W1015 12:40:00.123456       1 reflector.go:561] failed to list *v1.Pod: context deadline exceeded`,
}

// noMatchLogs returns n non-matching records for one pod.
func noMatchLogs(n int) plog.Logs {
	ld := plog.NewLogs()
	lrs := addResource(ld, vllmPodResource("vllm-0"))
	for i := range n {
		addRecord(lrs, base.Add(time.Duration(i)*time.Millisecond), noMatchBodies[i%len(noMatchBodies)])
	}
	return ld
}

// BenchmarkConsumeLogsNoMatch measures the hot path: a batch of 1000
// records, none of which matches the vLLM rule. It uses a Nop logger, as in
// production at the default log level.
func BenchmarkConsumeLogsNoMatch(b *testing.B) {
	const records = 1000
	conn, err := newConnector(connectortest.NewNopSettings(component.MustNewType(TypeStr)), loadVLLMRuleFile(b), consumertest.NewNop())
	require.NoError(b, err)
	ld := noMatchLogs(records)
	ld.MarkReadOnly()
	ctx := context.Background()
	b.ReportAllocs()
	n := 0
	for b.Loop() {
		if err := conn.ConsumeLogs(ctx, ld); err != nil {
			b.Fatal(err)
		}
		n++
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(n*records), "ns/record")
}

// vllmStartupLine is a startup line that the vLLM rule uses.
const vllmStartupLine = "INFO 10-15 12:00:00 [core.py:77] Initializing a V1 LLM engine (v0.11.0) with config: model='m', tensor_parallel_size=1, seed=0"

// trackedActivityLogs returns pods ResourceLogs with perPod non-matching
// records each. The pods are vllm-00, vllm-01, and so on.
func trackedActivityLogs(pods, perPod int) plog.Logs {
	ld := plog.NewLogs()
	for p := range pods {
		lrs := addResource(ld, vllmPodResource(fmt.Sprintf("vllm-%02d", p)))
		for i := range perPod {
			addRecord(lrs, base.Add(time.Duration(i)*time.Millisecond), noMatchBodies[(p+i)%len(noMatchBodies)])
		}
	}
	return ld
}

// newTrackedConnector returns a connector with the vLLM rule that tracks
// pods vllm-00 to vllm-<pods-1>, inserted an hour before base.
func newTrackedConnector(tb testing.TB, pods int) *infoMetricConnector {
	tb.Helper()
	conn, err := newConnector(connectortest.NewNopSettings(component.MustNewType(TypeStr)), loadVLLMRuleFile(tb), consumertest.NewNop())
	require.NoError(tb, err)
	ld := plog.NewLogs()
	for p := range pods {
		addRecord(addResource(ld, vllmPodResource(fmt.Sprintf("vllm-%02d", p))), base.Add(-time.Hour), vllmStartupLine)
	}
	require.NoError(tb, conn.ConsumeLogs(context.Background(), ld))
	require.Len(tb, conn.state.snapshot().Rules["vllm_engine_config_info"], pods)
	return conn
}

// BenchmarkConsumeLogsTrackedActivity measures the activity path: a batch of
// 1000 non-matching records over 20 ResourceLogs, each of a tracked pod.
// After the first call, the Activity of every pod is current, which is the
// steady state.
func BenchmarkConsumeLogsTrackedActivity(b *testing.B) {
	const (
		pods    = 20
		perPod  = 50
		records = pods * perPod
	)
	conn := newTrackedConnector(b, pods)
	ld := trackedActivityLogs(pods, perPod)
	ld.MarkReadOnly()
	ctx := context.Background()
	b.ReportAllocs()
	n := 0
	for b.Loop() {
		if err := conn.ConsumeLogs(ctx, ld); err != nil {
			b.Fatal(err)
		}
		n++
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(n*records), "ns/record")
}
