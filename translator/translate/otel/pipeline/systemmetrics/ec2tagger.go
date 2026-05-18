// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetrics

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

type ec2TaggerTranslator struct {
	factory processor.Factory
}

func newEc2TaggerTranslator() common.ComponentTranslator {
	return &ec2TaggerTranslator{factory: ec2tagger.NewFactory()}
}

func (t *ec2TaggerTranslator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), common.PipelineNameSystemMetrics)
}

func (t *ec2TaggerTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*ec2tagger.Config)
	credentials := confmap.NewFromStringMap(agent.Global_Config.Credentials)
	_ = credentials.Unmarshal(cfg)

	// Always add InstanceId
	cfg.EC2MetadataTags = []string{"InstanceId"}
	cfg.MiddlewareID = &agenthealth.StatusCodeID
	cfg.IMDSRetries = retryer.GetDefaultRetryNumber()

	return cfg, nil
}
