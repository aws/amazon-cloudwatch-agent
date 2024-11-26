// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

// intervalStats restricts the Stats get function to once
// per interval.
type intervalStats struct {
	mu       sync.RWMutex
	interval time.Duration

	getOnce *sync.Once
	lastGet time.Time

	stats atomic.Value
}

var _ agent.StatsProvider = (*intervalStats)(nil)

func (p *intervalStats) Stats(string) agent.Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var stats agent.Stats
	p.getOnce.Do(func() {
		p.lastGet = time.Now()
		stats = p.getStats()
		go p.allowNextGetAfter(p.interval)
	})
	return stats
}

func (p *intervalStats) getStats() agent.Stats {
	var stats agent.Stats
	if value := p.stats.Load(); value != nil {
		stats = value.(agent.Stats)
	}
	return stats
}

func (p *intervalStats) allowNextGetAfter(interval time.Duration) {
	time.Sleep(interval)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.getOnce = new(sync.Once)
}

func newIntervalStats(interval time.Duration) *intervalStats {
	return &intervalStats{
		getOnce:  new(sync.Once),
		interval: interval,
	}
}
