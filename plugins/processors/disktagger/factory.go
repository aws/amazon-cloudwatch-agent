// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/cloudprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/disktagger/azure"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/disktagger/internal/volume"
)

const typeStr = "disktagger"

func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		RefreshInterval:  5 * time.Minute,
		DiskDeviceTagKey: "device",
	}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (processor.Metrics, error) {
	c := cfg.(*Config)
	t := newTagger(c, set.Logger, newCacheFactory(ctx))
	return processorhelper.NewMetrics(ctx, set, cfg, next, t.processMetrics,
		processorhelper.WithStart(t.Start),
		processorhelper.WithShutdown(t.Shutdown),
	)
}

// cacheFactory creates volume.Cache based on cloud provider config.
// This allows the Tagger to create the cache in its Start method.
type cacheFactory func(cfg *Config) volume.Cache

func newCacheFactory(ctx context.Context) cacheFactory {
	return func(cfg *Config) volume.Cache {
		switch cfg.CloudProvider {
		case cloudprovider.AWS:
			credConfig := &configaws.CredentialsConfig{Region: cfg.Region}
			awsCfg, err := credConfig.LoadConfig(ctx)
			if err != nil {
				return nil
			}
			return volume.NewCache(volume.NewProvider(ec2.NewFromConfig(awsCfg), cfg.InstanceID))
		case cloudprovider.Azure:
			return volume.NewCache(azure.NewProvider())
		default:
			return nil
		}
	}
}
