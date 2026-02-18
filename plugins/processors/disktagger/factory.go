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
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata"
	"github.com/aws/amazon-cloudwatch-agent/internal/cloudprovider"
	awsprovider "github.com/aws/amazon-cloudwatch-agent/plugins/processors/disktagger/aws"
	azureprovider "github.com/aws/amazon-cloudwatch-agent/plugins/processors/disktagger/azure"
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
	provider := createDiskProvider(ctx, set)
	t := newTagger(c, set.Logger, provider)
	return processorhelper.NewMetrics(ctx, set, cfg, next, t.processMetrics,
		processorhelper.WithStart(t.Start),
		processorhelper.WithShutdown(t.Shutdown),
	)
}

func createDiskProvider(ctx context.Context, set processor.Settings) DiskProvider {
	p := cloudmetadata.GetProvider()
	if p == nil {
		set.Logger.Warn("No cloud provider detected, disktagger will not tag disks")
		return nil
	}

	switch p.CloudProvider() {
	case cloudprovider.AWS:
		credConfig := &configaws.CredentialsConfig{
			Region: p.Region(),
		}
		awsCfg, err := credConfig.LoadConfig(ctx)
		if err != nil {
			set.Logger.Warn("Failed to load AWS config for disktagger")
			return nil
		}
		set.Logger.Info("disktagger: using AWS EBS provider", zap.String("instanceID", p.InstanceID()), zap.String("region", p.Region()))
		return awsprovider.NewProvider(ec2.NewFromConfig(awsCfg), p.InstanceID())
	case cloudprovider.Azure:
		set.Logger.Info("disktagger: using Azure managed disk provider")
		ap := azureprovider.NewProvider()
		return newMapProvider(ap.DeviceToDiskID)
	default:
		set.Logger.Warn("Unsupported cloud provider for disktagger")
		return nil
	}
}
