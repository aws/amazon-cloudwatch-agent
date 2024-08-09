// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"errors"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/rules"
)

type Config struct {
	Resolvers []Resolver     `mapstructure:"resolvers"`
	Rules     []rules.Rule   `mapstructure:"rules"`
	Limiter   *LimiterConfig `mapstructure:"limiter"`
}

type LimiterConfig struct {
	Threshold                 int             `mapstructure:"drop_threshold"`
	Disabled                  bool            `mapstructure:"disabled"`
	LogDroppedMetrics         bool            `mapstructure:"log_dropped_metrics"`
	RotationInterval          time.Duration   `mapstructure:"rotation_interval"`
	GarbageCollectionInterval time.Duration   `mapstructure:"garbage_collection_interval"`
	ParentContext             context.Context `mapstructure:"-"`
}

const (
	DefaultThreshold        = 500
	DefaultRotationInterval = 1 * time.Hour
	DefaultGCInterval       = 10 * time.Minute
)

func NewDefaultLimiterConfig() *LimiterConfig {
	return &LimiterConfig{
		Threshold:                 DefaultThreshold,
		Disabled:                  false,
		LogDroppedMetrics:         false,
		RotationInterval:          DefaultRotationInterval,
		GarbageCollectionInterval: DefaultGCInterval,
	}
}

func (lc *LimiterConfig) Validate() {
	if lc.GarbageCollectionInterval == 0 {
		lc.GarbageCollectionInterval = DefaultGCInterval
	}
}

func (cfg *Config) Validate() error {
	if len(cfg.Resolvers) == 0 {
		return errors.New("resolvers must not be empty")
	}
	for _, resolver := range cfg.Resolvers {
		switch resolver.Platform {
		case PlatformEKS:
			if resolver.Name == "" {
				return errors.New("name must not be empty for eks resolver")
			}
		case PlatformK8s:
			if resolver.Name == "" {
				return errors.New("name must not be empty for k8s resolver")
			}
		case PlatformEC2, PlatformGeneric:
		case PlatformECS:
			return errors.New("ecs resolver is not supported")
		default:
			return errors.New("unknown resolver")
		}
	}

	if cfg.Limiter != nil {
		cfg.Limiter.Validate()
	}
	return nil
}
