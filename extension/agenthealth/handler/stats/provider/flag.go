// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

const (
	flagGetInterval = 5 * time.Minute
)

type Flag int

const (
	FlagIMDSFallbackSucceed = iota
	FlagSharedConfigFallback
	FlagAppSignal
	FlagEnhancedContainerInsights
)

var (
	flagSingleton FlagStats
	flagOnce      sync.Once
)

type FlagStats interface {
	agent.StatsProvider
	SetFlag(flag Flag)
}

type flagStats struct {
	*intervalStats

	flags sync.Map
}

var _ FlagStats = (*flagStats)(nil)

func (p *flagStats) update() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats = agent.Stats{
		ImdsFallbackSucceed:       p.getFlag(FlagIMDSFallbackSucceed),
		SharedConfigFallback:      p.getFlag(FlagSharedConfigFallback),
		AppSignals:                p.getFlag(FlagAppSignal),
		EnhancedContainerInsights: p.getFlag(FlagEnhancedContainerInsights),
	}
}

func (p *flagStats) getFlag(flag Flag) *int {
	if _, ok := p.flags.Load(flag); ok {
		return aws.Int(1)
	}
	return nil
}

func (p *flagStats) SetFlag(flag Flag) {
	if _, ok := p.flags.Load(flag); !ok {
		p.flags.Store(flag, true)
		p.update()
	}
}

func newFlagStats(interval time.Duration) *flagStats {
	return &flagStats{
		intervalStats: newIntervalStats(interval),
	}
}

func GetFlagsStats() FlagStats {
	flagOnce.Do(func() {
		flagSingleton = newFlagStats(flagGetInterval)
	})
	return flagSingleton
}
