// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/internal/entity"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
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
	transform                *entity.Transform
}

func NewTranslator() common.ComponentTranslator {
	return &translator{
		factory: awsentity.NewFactory(),
	}
}

func NewTranslatorWithEntityType(entityType string, name string, scrapeDatapointAttribute bool) common.ComponentTranslator {
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

func NewTranslatorWithEntityTypeAndTransform(entityType string, name string, scrapeDatapointAttribute bool, transform *entity.Transform) common.ComponentTranslator {
	pipelineName := strings.ToLower(entityType)
	if name != "" {
		pipelineName = pipelineName + "/" + name
	}

	return &translator{
		factory:                  awsentity.NewFactory(),
		entityType:               entityType,
		name:                     pipelineName,
		scrapeDatapointAttribute: scrapeDatapointAttribute,
		transform:                transform,
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	ctx := context.CurrentContext()

	// Do not send entity for ECS
	if ctx.RunInContainer() && ecsutil.GetECSUtilSingleton().IsECS() {
		return nil, nil
	}

	cfg := t.factory.CreateDefaultConfig().(*awsentity.Config)

	if t.entityType != "" {
		cfg.EntityType = t.entityType
	}

	if t.scrapeDatapointAttribute {
		cfg.ScrapeDatapointAttribute = true
	}

	cfg.KubernetesMode = ctx.KubernetesMode()
	// We want to keep platform config variable to be
	// anything that is non-Kubernetes related so the
	// processor can perform different logics for EKS
	// in EC2 or Non-EC2
	cfg.Platform = ctx.Mode()

	if t.transform != nil {
		cfg.TransformEntity = t.transform
	}

	if cfg.KubernetesMode != "" {
		clusterName, clusterNameConfigured := common.GetHostedIn(conf)

		if !clusterNameConfigured {
			clusterName = common.GetClusterName(conf)
		}

		cfg.ClusterName = clusterName
	}

	return cfg, nil
}
