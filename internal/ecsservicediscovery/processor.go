// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import "github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"

type Processor interface {
	Process(cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error)
	ProcessorName() string
}

type DefaultProcessor struct {
	configurer *awsmiddleware.Configurer
}

func (p *DefaultProcessor) GetConfigurer() *awsmiddleware.Configurer {
	return p.configurer
}
