// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package profiler

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

var (
	Profiler profiler = profiler{
		stats: make(map[string]float64),
	}
	noStatsInProfiler = "[no stats is available...]"
)

type profiler struct {
	sync.Mutex
	stats map[string]float64
}

// use slice for key is enough now, could be expand to map if we need dimensions
func (p *profiler) AddStats(key []string, value float64) {
	p.Lock()
	defer p.Unlock()
	k := strings.Join(key, "_")
	p.stats[k] += value
}

// GetStats for testing purposes
func (p *profiler) GetStats() map[string]float64 {
	p.Lock()
	defer p.Unlock()
	return p.stats
}

func (p *profiler) ReportAndClear() {
	p.Lock()
	defer p.Unlock()
	output := p.reportAndClear()
	log.Printf("D! Profiler dump:\n%s", strings.Join(output, "\n"))
}

func (p *profiler) reportAndClear() []string {
	var output []string
	for k, v := range p.stats {
		output = append(output, fmt.Sprintf("[%s: %f]", k, v))
		delete(p.stats, k)
	}

	if len(output) == 0 {
		output = append(output, noStatsInProfiler)
	}
	return output
}
