// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

const (
	flagGetInterval = 5 * time.Minute
)

var (
	flagSingleton FlagStats
	flagOnce      sync.Once
)

type FlagStats interface {
	agent.StatsProvider
	SetImdsFallbackSucceed()
	SetSharedConfigFallback()
}

type flagStats struct {
	*intervalStats

	sharedConfigFallback atomic.Bool
	imdsFallbackSucceed  atomic.Bool
}

var _ FlagStats = (*flagStats)(nil)

func (p *flagStats) update() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats = agent.Stats{
		ImdsFallbackSucceed:  p.getImdsFallbackSucceed(),
		SharedConfigFallback: p.getSharedConfigFallback(),
	}
}

func (p *flagStats) SetImdsFallbackSucceed() {
	if !p.imdsFallbackSucceed.Load() {
		p.imdsFallbackSucceed.Store(true)
		p.update()
	}
}

func (p *flagStats) getImdsFallbackSucceed() *int {
	if p.imdsFallbackSucceed.Load() {
		return aws.Int(1)
	}
	return nil
}

func (p *flagStats) SetSharedConfigFallback() {
	if !p.sharedConfigFallback.Load() {
		p.sharedConfigFallback.Store(true)
		p.update()
	}
}

func (p *flagStats) getSharedConfigFallback() *int {
	if p.sharedConfigFallback.Load() {
		return aws.Int(1)
	}
	return nil
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
