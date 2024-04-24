// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2

import "github.com/aws/amazon-cloudwatch-agent/internal/retryer"

type Option interface {
	apply(*Config)
}

type optionFunc func(*Config)

func (f optionFunc) apply(cfg *Config) {
	f(cfg)
}

func WithIMDSv2Retries(retries int) Option {
	return optionFunc(func(cfg *Config) {
		cfg.IMDSv2Retries = retries
	})
}

type Config struct {
	// IMDSv2Retries is the number of retries the IMDSv2 MetadataProvider will make before it errors out.
	IMDSv2Retries int
}

func (c *Config) WithOptions(opts ...Option) *Config {
	for _, opt := range opts {
		opt.apply(c)
	}
	return c
}

func DefaultConfig() *Config {
	return &Config{
		IMDSv2Retries: retryer.GetDefaultRetryNumber(),
	}
}
