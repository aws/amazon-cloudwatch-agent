// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// scopeName is the instrumentation scope of the emitted metrics, and the
// name of the meter of the connector's own telemetry.
const scopeName = "github.com/aws/amazon-cloudwatch-agent/connector/signaltoinfometricconnector"

// The first tick of each emit group comes startupEmitDelay plus a random
// jitter in [0, min(interval, maxStartupJitter)) after Start. The delay
// gives k8sattributes in the downstream metrics pipeline time to sync its
// pod cache; without it, a filter on its attributes drops every series of
// the first emission. The jitter spreads the ticks of the agents of a
// cluster that start together.
const (
	startupEmitDelay = 10 * time.Second
	maxStartupJitter = 10 * time.Second
)

// emitTrigger is what caused an emission.
type emitTrigger uint8

const (
	// triggerChange: emit_on_change, after a batch inserted or changed
	// entities.
	triggerChange emitTrigger = iota
	// triggerInterval: a tick of an emit group.
	triggerInterval

	numTriggers
)

// emitGroup is the rules that share one effective emit interval. Each group
// has one goroutine and one call to the next consumer per tick.
type emitGroup struct {
	interval time.Duration
	// rules are rule indexes, in config order.
	rules []int
}

// newEmitGroups groups the rules by effective emit interval, in the order
// of the first rule of each interval.
func newEmitGroups(cfg *Config) []emitGroup {
	var groups []emitGroup
	index := make(map[time.Duration]int)
	for i := range cfg.Rules {
		interval := cfg.Rules[i].effectiveEmitInterval(cfg.EmitInterval)
		gi, ok := index[interval]
		if !ok {
			gi = len(groups)
			index[interval] = gi
			groups = append(groups, emitGroup{interval: interval})
		}
		groups[gi].rules = append(groups[gi].rules, i)
	}
	return groups
}

// randomJitter returns a random duration in [0, n). n must be positive.
func randomJitter(n time.Duration) time.Duration {
	return rand.N(n) //nolint:gosec // G404: a start jitter does not need a secure random source.
}

// startEmitting launches one goroutine per emit group and returns at once.
func (c *infoMetricConnector) startEmitting() {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := range c.groups {
		g := &c.groups[i]
		delay := c.startupDelay + c.jitter(min(g.interval, maxStartupJitter))
		wg.Go(func() { c.runGroup(ctx, g, delay) })
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	c.stopEmit, c.emitDone = cancel, done
}

// cancelEmitting cancels the context of the emit groups and returns at
// once. It is safe to call before startEmitting and more than once.
func (c *infoMetricConnector) cancelEmitting() {
	if c.stopEmit != nil {
		c.stopEmit()
	}
}

// waitEmitting waits for the emit groups to stop after cancelEmitting,
// bounded by ctx. It is safe to call before startEmitting and more than
// once.
func (c *infoMetricConnector) waitEmitting(ctx context.Context) error {
	if c.emitDone == nil {
		return nil
	}
	select {
	case <-c.emitDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runGroup ticks after delay, then every interval, until ctx is done.
func (c *infoMetricConnector) runGroup(ctx context.Context, g *emitGroup, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()
	for {
		c.tick(ctx, g)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// tick emits every entity of the group's rules in one call to the next
// consumer. The entities are copied under the state lock, and the metrics
// are built and sent without it. An empty state sends nothing.
func (c *infoMetricConnector) tick(ctx context.Context, g *emitGroup) {
	c.emit(ctx, c.state.entries(g.rules), triggerInterval)
}

// buildMetrics returns one ResourceMetrics per entry. The resource has the
// included resource attributes. The single gauge data point has the value
// 1, the facts as attributes in config order, the emit time as Timestamp,
// and FirstSeen as StartTimestamp. The record time is never the Timestamp,
// because CloudWatch rejects data points older than 14 days.
func (c *infoMetricConnector) buildMetrics(now time.Time, entries []emitEntry) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	rms.EnsureCapacity(len(entries))
	ts := pcommon.NewTimestampFromTime(now)
	nowNanos := now.UnixNano()
	for i := range entries {
		e := &entries[i].entity
		rule := c.rules[entries[i].rule].cfg

		rm := rms.AppendEmpty()
		res := rm.Resource().Attributes()
		res.EnsureCapacity(len(e.Resource))
		for _, p := range e.Resource {
			res.PutStr(p.Key, p.Value)
		}

		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName(scopeName)
		m := sm.Metrics().AppendEmpty()
		m.SetName(rule.Name)
		m.SetDescription(rule.Description)
		m.SetUnit(rule.Unit)
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(1)
		dp.SetTimestamp(ts)
		// A record time ahead of the emit time (clock skew) must not give a
		// start time after the data point time.
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, min(e.FirstSeen, nowNanos))))
		attrs := dp.Attributes()
		attrs.EnsureCapacity(len(e.Facts))
		for _, p := range e.Facts {
			attrs.PutStr(p.Key, p.Value)
		}
	}
	return md
}

// emit builds the metrics for entries, with the current time as the data
// point time, and sends them to the next consumer in one call. A downstream
// error is logged at most once per logInterval and not returned: the logs
// pipeline that the connector shares with other exporters must not fail
// because of the metrics side, and a tick has no caller to return it to.
//
// An error after ctx ended is logged at Debug and not counted: Shutdown
// cancels the context of a tick that is in progress, and the next consumer
// then returns the cancellation, which is not a downstream failure.
func (c *infoMetricConnector) emit(ctx context.Context, entries []emitEntry, trigger emitTrigger) {
	if len(entries) == 0 {
		return
	}
	err := c.next.ConsumeMetrics(ctx, c.buildMetrics(c.now(), entries))
	if err != nil && ctx.Err() != nil {
		if ce := c.logger.Check(zap.DebugLevel, "Info metrics not sent: the emission was canceled"); ce != nil {
			ce.Write(zap.Error(err), zap.String("trigger", triggerNames[trigger]))
		}
		return
	}
	c.tel.recordEmit(ctx, entries, trigger, err)
	if err != nil {
		c.logDownstreamError(err)
	}
}

func (c *infoMetricConnector) logDownstreamError(err error) {
	suppressed, ok := c.downstreamErrLog.allow(c.now().UnixNano(), 1)
	if !ok {
		return
	}
	c.logger.Warn("Failed to send Info metrics to the next consumer",
		zap.Error(err),
		zap.Int64("suppressed_since_last_log", suppressed),
	)
}
