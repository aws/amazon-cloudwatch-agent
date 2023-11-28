// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

const (
	flagGetInterval = 5 * time.Minute
)

type Flag int

const (
	FlagIMDSFallbackSucceed Flag = iota
	FlagSharedConfigFallback
	FlagAppSignal
	FlagEnhancedContainerInsights
	FlagRunningInContainer
	FlagMode
	FlagRegionType
)

var (
	flagSingleton FlagStats
	flagOnce      sync.Once
)

type FlagStats interface {
	agent.StatsProvider
	SetFlag(flag Flag)
	SetFlagWithValue(flag Flag, value string)
}

type flagStats struct {
	*intervalStats

	flags sync.Map
}

var _ FlagStats = (*flagStats)(nil)

func (p *flagStats) update() {
	p.stats.Store(agent.Stats{
		ImdsFallbackSucceed:       p.getIntFlag(FlagIMDSFallbackSucceed, false),
		SharedConfigFallback:      p.getIntFlag(FlagSharedConfigFallback, false),
		AppSignals:                p.getIntFlag(FlagAppSignal, false),
		EnhancedContainerInsights: p.getIntFlag(FlagEnhancedContainerInsights, false),
		RunningInContainer:        p.getIntFlag(FlagRunningInContainer, true),
		Mode:                      p.getStringFlag(FlagMode),
		RegionType:                p.getStringFlag(FlagRegionType),
	})
}

func (p *flagStats) getIntFlag(flag Flag, missingAsZero bool) *int {
	if _, ok := p.flags.Load(flag); ok {
		return aws.Int(1)
	}
	if missingAsZero {
		return aws.Int(0)
	}
	return nil
}

func (p *flagStats) getStringFlag(flag Flag) *string {
	value, ok := p.flags.Load(flag)
	if !ok {
		return nil
	}
	var str string
	str, ok = value.(string)
	if !ok {
		return nil
	}
	return aws.String(str)
}

func (p *flagStats) SetFlag(flag Flag) {
	if _, ok := p.flags.Load(flag); !ok {
		p.flags.Store(flag, true)
		p.update()
	}
}

func (p *flagStats) SetFlagWithValue(flag Flag, value string) {
	if _, ok := p.flags.Load(flag); !ok {
		p.flags.Store(flag, value)
		p.update()
	}
}

func newFlagStats(interval time.Duration) *flagStats {
	stats := &flagStats{
		intervalStats: newIntervalStats(interval),
	}
	if envconfig.IsRunningInContainer() {
		stats.SetFlag(FlagRunningInContainer)
	}
	return stats
}

func GetFlagsStats() FlagStats {
	flagOnce.Do(func() {
		flagSingleton = newFlagStats(flagGetInterval)
	})
	return flagSingleton
}
