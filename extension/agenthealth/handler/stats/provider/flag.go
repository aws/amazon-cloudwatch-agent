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

var (
	flagSingleton *flagStats
	flagOnce      sync.Once
)

type flagStats struct {
	*intervalStats

	flagSet agent.FlagSet
}

func (p *flagStats) update() {
	p.stats.Store(agent.Stats{
		ImdsFallbackSucceed:       boolToSparseInt(p.flagSet.IsSet(agent.FlagIMDSFallbackSuccess)),
		SharedConfigFallback:      boolToSparseInt(p.flagSet.IsSet(agent.FlagSharedConfigFallback)),
		AppSignals:                boolToSparseInt(p.flagSet.IsSet(agent.FlagAppSignal)),
		EnhancedContainerInsights: boolToSparseInt(p.flagSet.IsSet(agent.FlagEnhancedContainerInsights)),
		RunningInContainer:        boolToInt(p.flagSet.IsSet(agent.FlagRunningInContainer)),
		Mode:                      p.flagSet.GetString(agent.FlagMode),
		RegionType:                p.flagSet.GetString(agent.FlagRegionType),
	})
}

func boolToInt(value bool) *int {
	result := boolToSparseInt(value)
	if result != nil {
		return result
	}
	return aws.Int(0)
}

func boolToSparseInt(value bool) *int {
	if value {
		return aws.Int(1)
	}
	return nil
}

func newFlagStats(flagSet agent.FlagSet, interval time.Duration) *flagStats {
	stats := &flagStats{
		flagSet:       flagSet,
		intervalStats: newIntervalStats(interval),
	}
	stats.flagSet.OnChange(stats.update)
	if envconfig.IsRunningInContainer() {
		stats.flagSet.Set(agent.FlagRunningInContainer)
	} else {
		stats.update()
	}
	return stats
}

func GetFlagsStats() agent.StatsProvider {
	flagOnce.Do(func() {
		flagSingleton = newFlagStats(agent.UsageFlags(), flagGetInterval)
	})
	return flagSingleton
}
