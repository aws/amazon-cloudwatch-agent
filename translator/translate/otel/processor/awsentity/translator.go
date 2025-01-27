// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"os"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	name     = "awsentity"
	Service  = "Service"
	Resource = "Resource"
)

type translator struct {
	factory                  processor.Factory
	entityType               string
	name                     string
	scrapeDatapointAttribute bool
}

func NewTranslator() common.Translator[component.Config] {
	return &translator{
		factory: awsentity.NewFactory(),
	}
}

func NewTranslatorWithEntityType(entityType string, name string, scrapeDatapointAttribute bool) common.Translator[component.Config] {
	pipelineName := strings.ToLower(entityType)
	if name != "" {
		pipelineName = pipelineName + "/" + name
	}

	return &translator{
		factory:                  awsentity.NewFactory(),
		entityType:               entityType,
		name:                     pipelineName,
		scrapeDatapointAttribute: scrapeDatapointAttribute,
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	// Do not send entity for ECS
	if context.CurrentContext().RunInContainer() && ecsutil.GetECSUtilSingleton().IsECS() {
		return nil, nil
	}

	cfg := t.factory.CreateDefaultConfig().(*awsentity.Config)

	if t.entityType != "" {
		cfg.EntityType = t.entityType
	}

	if t.scrapeDatapointAttribute {
		cfg.ScrapeDatapointAttribute = true
	}

	searchKeys := []string{
		common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.AppSignals, "hosted_in"),
		common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.AppSignalsFallback, "hosted_in"),
		common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.OtlpKey, "cluster_name"),
		common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.OtlpKey, "cluster_name"),
	}

	var clusterName string
	var found bool

	for _, path := range searchKeys {
		val, ok := common.GetString(conf, path)
		if ok && val != "" {
			clusterName = val
			found = true
			break
		}
	}

	//TODO: This logic is more or less identical to what AppSignals does. This should be moved to a common place for reuse
	ctx := context.CurrentContext()
	cfg.KubernetesMode = ctx.KubernetesMode()

	if !found && cfg.KubernetesMode != "" {
		envVarClusterName := os.Getenv("K8S_CLUSTER_NAME")
		if envVarClusterName != "" {
			clusterName = envVarClusterName
			found = true
		}
	}

	if !found {
		clusterName = util.GetClusterNameFromEc2Tagger()
	}

	if cfg.KubernetesMode != "" {
		cfg.ClusterName = clusterName
	}

	// We want to keep platform config variable to be
	// anything that is non-Kubernetes related so the
	// processor can perform different logics for EKS
	// in EC2 or Non-EC2
	cfg.Platform = ctx.Mode()
	return cfg, nil
}
