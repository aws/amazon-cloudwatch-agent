// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestBuildMetrics(t *testing.T) {
	cfg := testConfig()
	cfg.Rules[0].Identity = []string{"k8s.pod.uid", "k8s.container.name"}
	cfg.Rules[0].IncludeResourceAttributes = []string{"k8s.pod.name", "k8s.namespace.name", "k8s.pod.uid", "k8s.container.name"}
	second := cfg.Rules[0]
	second.Name = "other_info"
	second.Description = ""
	second.Unit = ""
	cfg.Rules = append(cfg.Rules, second)

	first := base.UnixNano()
	entries := []emitEntry{
		{rule: 0, entity: entity{
			Identity:  []kv{{"k8s.pod.uid", "u1"}, {"k8s.container.name", "c1"}},
			Resource:  []kv{{"k8s.pod.name", "p1"}, {"k8s.namespace.name", "ns"}, {"k8s.pod.uid", "u1"}, {"k8s.container.name", "c1"}},
			Facts:     []kv{{"z", "1"}, {"a", "2"}, {"m", "3"}},
			FirstSeen: first,
			Updated:   first + 10,
			Activity:  first + 20,
		}},
		// The namespace was absent, and a FirstSeen after the emit time
		// (clock skew) is clamped.
		{rule: 1, entity: entity{
			Identity:  []kv{{"k8s.pod.uid", "u2"}, {"k8s.container.name", "c2"}},
			Resource:  []kv{{"k8s.pod.name", "p2"}, {"k8s.pod.uid", "u2"}, {"k8s.container.name", "c2"}},
			Facts:     []kv{{"z", "9"}},
			FirstSeen: testNow.Add(time.Hour).UnixNano(),
		}},
	}

	conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
	md := conn.buildMetrics(testNow, entries)
	got := allSeries(t, []pmetric.Metrics{md})
	require.Len(t, got, 2)
	assert.Equal(t, series{
		resource:       []kv{{"k8s.pod.name", "p1"}, {"k8s.namespace.name", "ns"}, {"k8s.pod.uid", "u1"}, {"k8s.container.name", "c1"}},
		name:           "app_info",
		start:          base,
		ts:             testNow,
		attrs:          []kv{{"z", "1"}, {"a", "2"}, {"m", "3"}},
		value:          1,
		scopeName:      "github.com/aws/amazon-cloudwatch-agent/connector/signaltoinfometricconnector",
		description:    "App configuration.",
		unit:           "1",
		dataPointCount: 2,
	}, got[0])
	assert.Equal(t, series{
		resource:       []kv{{"k8s.pod.name", "p2"}, {"k8s.pod.uid", "u2"}, {"k8s.container.name", "c2"}},
		name:           "other_info",
		start:          testNow,
		ts:             testNow,
		attrs:          []kv{{"z", "9"}},
		value:          1,
		scopeName:      scopeName,
		dataPointCount: 2,
	}, got[1])

	conn, _ = newTestConnector(t, testConfig(), consumertest.NewNop())
	assert.Equal(t, 0, conn.buildMetrics(testNow, nil).ResourceMetrics().Len())
}

func TestEmitSkipsEmptyEntries(t *testing.T) {
	next := &countingConsumer{}
	conn, _ := newTestConnector(t, testConfig(), next)
	conn.emit(t.Context(), nil, triggerChange)
	assert.Equal(t, 0, next.calls())
}

func TestNewEmitGroups(t *testing.T) {
	cfg := testConfig()
	cfg.EmitInterval = 10 * time.Second
	rule := func(name string, interval time.Duration) RuleConfig {
		r := cfg.Rules[0]
		r.Name = name
		r.EmitInterval = interval
		return r
	}
	cfg.Rules = []RuleConfig{
		rule("a", 0),
		rule("b", 25*time.Second),
		// The connector interval set on the rule is the same group as 0.
		rule("c", 10*time.Second),
		rule("d", 25*time.Second),
		rule("e", time.Second),
	}
	assert.Equal(t, []emitGroup{
		{interval: 10 * time.Second, rules: []int{0, 2}},
		{interval: 25 * time.Second, rules: []int{1, 3}},
		{interval: time.Second, rules: []int{4}},
	}, newEmitGroups(cfg))
}

func TestRandomJitter(t *testing.T) {
	for range 100 {
		j := randomJitter(maxStartupJitter)
		assert.GreaterOrEqual(t, j, time.Duration(0))
		assert.Less(t, j, maxStartupJitter)
	}
	assert.Equal(t, time.Duration(0), randomJitter(1))
}

// newTickConnector returns a connector for a synctest bubble: its clock is
// the bubble clock, and its jitter returns jitter and records its argument.
func newTickConnector(t *testing.T, cfg *Config, next consumer.Metrics, jitter time.Duration) (*infoMetricConnector, *observer.ObservedLogs, *[]time.Duration) {
	t.Helper()
	conn, logs := newTestConnector(t, cfg, next)
	conn.now = time.Now
	var args []time.Duration
	conn.jitter = func(n time.Duration) time.Duration {
		args = append(args, n)
		return jitter
	}
	return conn, logs, &args
}

// metricNames returns the metric names of one ConsumeMetrics call, in order.
func metricNames(md pmetric.Metrics) []string {
	var names []string
	for _, rm := range md.ResourceMetrics().All() {
		for _, sm := range rm.ScopeMetrics().All() {
			for _, m := range sm.Metrics().All() {
				names = append(names, m.Name())
			}
		}
	}
	return names
}

func TestEmitTicks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := testConfig()
		// The ticks still emit without emit_on_change.
		cfg.EmitOnChange = false
		cfg.EmitInterval = 30 * time.Second
		sink := new(consumertest.MetricsSink)
		conn, _, jitterArgs := newTickConnector(t, cfg, sink, 3*time.Second)

		start := time.Now().UTC()
		require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
		assert.Equal(t, []time.Duration{maxStartupJitter}, *jitterArgs, "the jitter is below min(interval, 10s)")
		require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", start, "app started version=1")))
		assert.Empty(t, sink.AllMetrics(), "emit_on_change is false")

		first := start.Add(startupEmitDelay + 3*time.Second)
		time.Sleep(first.Sub(start) - time.Nanosecond)
		synctest.Wait()
		assert.Empty(t, sink.AllMetrics(), "no tick before the startup delay and the jitter")

		time.Sleep(time.Nanosecond)
		synctest.Wait()
		got := allSeries(t, sink.AllMetrics())
		require.Len(t, got, 1)
		assert.Equal(t, first, got[0].ts, "the data point time is the tick time")
		assert.Equal(t, start, got[0].start)
		assert.Equal(t, []kv{{"version", "1"}, {"mode", "default"}}, got[0].attrs)

		for i := 1; i <= 3; i++ {
			time.Sleep(cfg.EmitInterval - time.Nanosecond)
			synctest.Wait()
			require.Len(t, sink.AllMetrics(), i, "no tick before the interval")
			time.Sleep(time.Nanosecond)
			synctest.Wait()
			require.Len(t, sink.AllMetrics(), i+1)
			got := allSeries(t, sink.AllMetrics()[i:])
			require.Len(t, got, 1)
			assert.Equal(t, first.Add(time.Duration(i)*cfg.EmitInterval), got[0].ts)
			assert.Equal(t, start, got[0].start, "the series keeps its start time")
		}
		require.NoError(t, conn.Shutdown(t.Context()))
	})
}

func TestEmitJitterBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := testConfig()
		cfg.EmitInterval = 4 * time.Second
		conn, _, jitterArgs := newTickConnector(t, cfg, consumertest.NewNop(), 0)
		require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
		assert.Equal(t, []time.Duration{4 * time.Second}, *jitterArgs, "an interval below 10s bounds the jitter")
		require.NoError(t, conn.Shutdown(t.Context()))
	})
}

func TestEmitGroupsTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := testConfig()
		cfg.EmitOnChange = false
		cfg.EmitInterval = 10 * time.Second
		slow := cfg.Rules[0]
		slow.Name = "slow_info"
		slow.EmitInterval = 25 * time.Second
		same := cfg.Rules[0]
		same.Name = "same_info"
		same.EmitInterval = 10 * time.Second
		cfg.Rules = append(cfg.Rules, slow, same)
		sink := new(consumertest.MetricsSink)
		conn, _, jitterArgs := newTickConnector(t, cfg, sink, 0)

		start := time.Now()
		require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
		assert.Len(t, *jitterArgs, 2, "one jitter per group")
		// One record gives an entity to each of the three rules.
		require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", start, "app started version=1")))

		// calls returns the metric names of each call since the previous
		// check.
		seen := 0
		calls := func() [][]string {
			var out [][]string
			for _, md := range sink.AllMetrics()[seen:] {
				out = append(out, metricNames(md))
			}
			seen = len(sink.AllMetrics())
			return out
		}
		at := func(d time.Duration) {
			time.Sleep(time.Until(start.Add(d)))
			synctest.Wait()
		}

		at(startupEmitDelay)
		// The groups run concurrently, so the order of their calls is not fixed.
		assert.ElementsMatch(t, [][]string{{"app_info", "same_info"}, {"slow_info"}}, calls(),
			"rules that share an interval share one call")
		at(startupEmitDelay + 10*time.Second)
		assert.Equal(t, [][]string{{"app_info", "same_info"}}, calls())
		at(startupEmitDelay + 20*time.Second)
		assert.Equal(t, [][]string{{"app_info", "same_info"}}, calls())
		at(startupEmitDelay + 25*time.Second)
		assert.Equal(t, [][]string{{"slow_info"}}, calls())
		at(startupEmitDelay + 30*time.Second)
		assert.Equal(t, [][]string{{"app_info", "same_info"}}, calls())
		require.NoError(t, conn.Shutdown(t.Context()))
	})
}

func TestEmitTickEntriesSorted(t *testing.T) {
	cfg := testConfig()
	second := cfg.Rules[0]
	second.Name = "other_info"
	cfg.Rules = append(cfg.Rules, second)
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, cfg, sink)
	ld := podLogs("u3", base, "app started version=1")
	addRecord(addResource(ld, podResource("u1")), base, "app started version=1")
	addRecord(addResource(ld, podResource("u2")), base, "app started version=1")
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	sink.Reset()

	conn.tick(t.Context(), &emitGroup{interval: time.Minute, rules: []int{1, 0}})
	require.Len(t, sink.AllMetrics(), 1)
	var got []string
	for _, s := range allSeries(t, sink.AllMetrics()) {
		got = append(got, s.name+"/"+s.resource[0].Value)
	}
	assert.Equal(t, []string{
		"app_info/pod-u1", "app_info/pod-u2", "app_info/pod-u3",
		"other_info/pod-u1", "other_info/pod-u2", "other_info/pod-u3",
	}, got, "sorted by rule and then by identity")
}

func TestEmitTickDownstreamErrorIsSwallowed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := testConfig()
		cfg.EmitOnChange = false
		cfg.EmitInterval = 10 * time.Second
		next := &countingConsumer{err: errors.New("downstream unavailable")}
		conn, logs, _ := newTickConnector(t, cfg, next, 0)
		require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
		require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", time.Now(), "app started version=1")))

		time.Sleep(startupEmitDelay + 5*time.Minute)
		synctest.Wait()
		assert.Equal(t, 31, next.calls(), "the group keeps ticking after errors")
		warnings := logs.FilterMessage("Failed to send Info metrics to the next consumer").All()
		require.Len(t, warnings, 6, "one warning per minute")
		assert.Equal(t, int64(0), warnings[0].ContextMap()["suppressed_since_last_log"])
		assert.Equal(t, int64(5), warnings[1].ContextMap()["suppressed_since_last_log"])
		require.NoError(t, conn.Shutdown(t.Context()))
	})
}

func TestEmitEmptyStateSendsNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := testConfig()
		cfg.EmitInterval = 10 * time.Second
		next := &countingConsumer{}
		conn, _, _ := newTickConnector(t, cfg, next, 0)
		require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
		time.Sleep(time.Hour)
		synctest.Wait()
		assert.Equal(t, 0, next.calls())

		// Only records that fail body_contains: still nothing to emit.
		require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", time.Now(), "noise")))
		time.Sleep(time.Hour)
		synctest.Wait()
		assert.Equal(t, 0, next.calls())
		require.NoError(t, conn.Shutdown(t.Context()))
	})
}

func TestEmitShutdown(t *testing.T) {
	// Shutdown without Start, and twice.
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.Shutdown(t.Context()))
	require.NoError(t, conn.Shutdown(t.Context()))

	synctest.Test(t, func(t *testing.T) {
		cfg := testConfig()
		cfg.EmitOnChange = false
		cfg.EmitInterval = 10 * time.Second
		next := &countingConsumer{}
		conn, _, _ := newTickConnector(t, cfg, next, 0)
		require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
		require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", time.Now(), "app started version=1")))
		time.Sleep(startupEmitDelay + 10*time.Second)
		synctest.Wait()
		require.Equal(t, 2, next.calls())

		require.NoError(t, conn.Shutdown(t.Context()))
		time.Sleep(time.Hour)
		synctest.Wait()
		assert.Equal(t, 2, next.calls(), "no tick after Shutdown")
		require.NoError(t, conn.Shutdown(t.Context()), "a second Shutdown is safe")
	})

	// Shutdown before the first tick.
	synctest.Test(t, func(t *testing.T) {
		next := &countingConsumer{}
		conn, _, _ := newTickConnector(t, testConfig(), next, 0)
		require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
		require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", time.Now(), "app started version=1")))
		require.Equal(t, 1, next.calls(), "the emission on change")
		require.NoError(t, conn.Shutdown(t.Context()))
		time.Sleep(time.Hour)
		synctest.Wait()
		assert.Equal(t, 1, next.calls())
	})
}

// blockingConsumer blocks every ConsumeMetrics call until release is closed,
// whatever its context.
type blockingConsumer struct {
	release chan struct{}
	mu      sync.Mutex
	n       int
}

func (c *blockingConsumer) ConsumeMetrics(context.Context, pmetric.Metrics) error {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	<-c.release
	return nil
}

func (*blockingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *blockingConsumer) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func TestEmitShutdownBoundedByContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := testConfig()
		cfg.EmitOnChange = false
		next := &blockingConsumer{release: make(chan struct{})}
		conn, _, _ := newTickConnector(t, cfg, next, 0)
		require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
		require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", time.Now(), "app started version=1")))
		time.Sleep(startupEmitDelay)
		synctest.Wait()
		require.Equal(t, 1, next.calls(), "the first tick is in the next consumer")

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		start := time.Now()
		assert.ErrorIs(t, conn.Shutdown(ctx), context.DeadlineExceeded)
		assert.Equal(t, time.Second, time.Since(start))

		close(next.release)
		require.NoError(t, conn.Shutdown(context.Background()), "the group stops once the call returns")
		assert.Equal(t, 1, next.calls())
	})
}

// TestEmitCanceledIsNotAnError checks that a call to the next consumer that
// fails because the emission context ended is logged at Debug and not
// counted: a tick canceled by Shutdown, and an emission on change whose
// ConsumeLogs context ended.
func TestEmitCanceledIsNotAnError(t *testing.T) {
	const msg = "Info metrics not sent: the emission was canceled"
	t.Run("tick", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			cfg := testConfig()
			cfg.EmitOnChange = false
			next := &ctxConsumer{}
			conn, tel, logs := newTelemetryConnector(t, cfg, next)
			conn.jitter = func(time.Duration) time.Duration { return 0 }
			require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
			require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")))
			time.Sleep(startupEmitDelay)
			synctest.Wait()
			require.Equal(t, 1, next.calls(), "the first tick is in the next consumer")

			require.NoError(t, conn.Shutdown(context.Background()))
			got := collect(t, tel)
			assert.NotContains(t, got, metricEmitErrors)
			assert.NotContains(t, got, metricDatapointsEmitted)
			assert.Zero(t, logs.FilterLevelExact(zapcore.WarnLevel).Len())
			debugs := logs.FilterMessage(msg).All()
			require.Len(t, debugs, 1)
			assert.Equal(t, zapcore.DebugLevel, debugs[0].Level)
			assert.Equal(t, "context canceled", debugs[0].ContextMap()["error"])
			assert.Equal(t, "interval", debugs[0].ContextMap()["trigger"])
		})
	})

	t.Run("change", func(t *testing.T) {
		next := &ctxConsumer{}
		conn, tel, logs := newTelemetryConnector(t, testConfig(), next)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base, "app started version=1")))
		require.Equal(t, 1, next.calls())
		assert.NotContains(t, collect(t, tel), metricEmitErrors)
		assert.Zero(t, logs.FilterLevelExact(zapcore.WarnLevel).Len())
		require.Equal(t, 1, logs.FilterMessage(msg).Len())
		assert.Equal(t, "change", logs.FilterMessage(msg).All()[0].ContextMap()["trigger"])
		assert.Len(t, conn.state.snapshot().Rules["app_info"], 1, "the state is still updated")
	})
}
