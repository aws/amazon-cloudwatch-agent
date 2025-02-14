// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sattributesprocessor

import (
	_ "embed"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed k8sattributes_nodefilter.yaml
var k8sAttributesNodeFilterConfig string

//go:embed k8sattributes.yaml
var k8sAttributesConfig string

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, k8sattributesprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*k8sattributesprocessor.Config)
	currentContext := context.CurrentContext()

	if currentContext.KubernetesMode() == "" {
		return nil, fmt.Errorf("k8sattributesprocessor is only supported on kubernetes")
	}

	switch workloadType := currentContext.WorkloadType(); workloadType {
	case config.DaemonSet:
		return common.GetYamlFileToYamlConfig(cfg, k8sAttributesNodeFilterConfig)
	case config.Deployment, config.StatefulSet:
		return common.GetYamlFileToYamlConfig(cfg, k8sAttributesConfig)
	default:
		return nil, fmt.Errorf("k8sattributesprocessor is not supported for this workload type")
	}
}
