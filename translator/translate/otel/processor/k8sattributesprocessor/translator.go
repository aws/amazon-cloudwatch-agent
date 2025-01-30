// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sattributesprocessor

import (
	_ "embed"
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"
)

//go:embed k8sattributes_agent.yaml
var k8sAttributesAgentConfig string

//go:embed k8sattributes_gateway.yaml
var k8sAttributesGatewayConfig string

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, k8sattributesprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*k8sattributesprocessor.Config)
	ctx := context.CurrentContext()
	if ctx.KubernetesMode() != "" {
		workloadType := ctx.WorkloadType()
		if workloadType == "Deployment" || workloadType == "StatefulSet" {
			return common.GetYamlFileToYamlConfig(cfg, k8sAttributesGatewayConfig)
		} else if workloadType == "DaemonSet" {
			return common.GetYamlFileToYamlConfig(cfg, k8sAttributesAgentConfig)
		}
		return nil, fmt.Errorf("k8sattributesprocessor is not supported for this workload type")
	}
	return nil, fmt.Errorf("k8sattributesprocessor is not supported in this context")
}
