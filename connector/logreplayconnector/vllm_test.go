// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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

// TestVLLMRuleFile pins testdata/vllm_rule.yaml, which is the configuration
// that the Helm chart renders.
func TestVLLMRuleFile(t *testing.T) {
	cfg := loadVLLMRuleFile(t)
	assert.Equal(t, &Config{
		Interval:            time.Hour,
		SourceFileAttribute: "log.file.path",
		MaxStateBytes:       defaultMaxStateBytes,
		MaxRecordBytes:      defaultMaxRecordBytes,
		ErrorMode:           defaultErrorMode,
		Rules: []RuleConfig{{
			Name:         "vllm_engine_config",
			BodyContains: []string{"Initializing an LLM engine (v", "Initializing a V0 LLM engine (v", "Initializing a V1 LLM engine (v"},
			Identity:     []string{"k8s.namespace.name", "k8s.pod.name", "k8s.container.name"},
		}},
	}, cfg)
}

// TestREADMEExample checks that the example in README.md is a valid
// configuration and that its connector settings and rule are the ones in
// testdata/vllm_rule.yaml.
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
	cfg.Storage = nil
	assert.Equal(t, loadVLLMRuleFile(t), cfg)
}

// fixture is one file of testdata/vllm, a container log in CRI format.
type fixture struct {
	file, pod, namespace, container string
	lines                           []criLine
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
		f := fixture{
			file:      "/var/log/containers/" + filepath.Base(path),
			pod:       parts[0],
			namespace: parts[1],
			container: parts[2][:dash],
		}
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		for _, line := range strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), "\n") {
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

// fixtureLogs returns all fixtures as one batch, one ResourceLogs per file,
// with the resource and record attributes that the chart's filelog receiver
// and k8sattributes give.
func fixtureLogs(fixtures []fixture) plog.Logs {
	ld := plog.NewLogs()
	for i := range fixtures {
		f := &fixtures[i]
		sl := addResource(ld, map[string]string{
			"k8s.namespace.name": f.namespace,
			"k8s.pod.name":       f.pod,
			"k8s.container.name": f.container,
			"k8s.pod.uid":        "uid-" + f.pod,
		})
		for _, line := range f.lines {
			addRecord(sl, line.ts, line.body, f.file)
		}
	}
	return ld
}

// TestVLLMFixtures runs the chart's rule on container logs of five vLLM
// releases, v0.6.3 to v0.31.1. It keeps exactly the engine startup line of
// each container, not the "non-default args" line or other lines that
// mention the engine, and one tick sends one exact copy of each.
func TestVLLMFixtures(t *testing.T) {
	fixtures := loadFixtures(t)
	sink := new(consumertest.LogsSink)
	conn, _ := newTestConnector(t, loadVLLMRuleFile(t), sink)
	files := newFakeFiles()
	conn.checkSource = files.stat
	ld := fixtureLogs(fixtures)
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

	kept := conn.state.entries()
	require.Len(t, kept, 5)
	var want []plog.Logs
	versions := map[string]string{}
	for i := range fixtures {
		f := &fixtures[i]
		startup := -1
		for k, line := range f.lines {
			if strings.Contains(line.body, "LLM engine (v") && strings.Contains(line.body, ") with config: ") {
				require.Equal(t, -1, startup, "one startup line per fixture")
				startup = k
			}
		}
		require.NotEqual(t, -1, startup, f.file)
		want = append(want, copyOf(ld, i, startup, testNow))
		version := strings.SplitN(strings.SplitN(f.lines[startup].body, "LLM engine (v", 2)[1], ")", 2)[0]
		versions[f.pod] = version
	}
	assert.Equal(t, map[string]string{
		"vllm-main-7d9f":  "0.31.1",
		"vllm-v0285-5c4b": "0.28.0",
		"vllm-v0290-6a1e": "0.29.0",
		"vllm-v063-1b7c":  "0.6.3",
		"vllm-v085-9e2a":  "0.8.5",
	}, versions)

	conn.tick(t.Context())
	require.Len(t, sink.AllLogs(), 1)
	got := sink.AllLogs()[0]
	require.Equal(t, 5, got.LogRecordCount())
	// The copies come sorted by state key. Compare them by file.
	byFile := func(ld plog.Logs) map[string]plog.Logs {
		out := map[string]plog.Logs{}
		for i := range ld.ResourceLogs().Len() {
			one := plog.NewLogs()
			ld.ResourceLogs().At(i).CopyTo(one.ResourceLogs().AppendEmpty())
			path, ok := one.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("log.file.path")
			require.True(t, ok)
			out[path.Str()] = one
		}
		return out
	}
	gotByFile := byFile(got)
	wantByFile := byFile(mergeLogs(want...))
	require.Len(t, gotByFile, 5)
	for file, w := range wantByFile {
		requireLogsEqual(t, w, gotByFile[file])
		assert.Equal(t, 1, files.count(file), "the tick checks each source file once")
	}
}
