// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
)

const (
	attributeAwsLogGroupNames      = "aws.log.group.names"
	attributeDeploymentEnvironment = "deployment.environment"
	attributeServiceName           = "service.name"
)

// exposed as a variable for unit testing
var addToEntityStore = func(logGroupName entitystore.LogGroupName, serviceName string, environmentName string) {
	rs := entitystore.GetEntityStore()
	if rs == nil {
		return
	}
	rs.AddServiceAttrEntryForLogGroup(logGroupName, serviceName, environmentName)
}

// awsEntityProcessor looks for metrics that have the aws.log.group.names and either the service.name or
// deployment.environment resource attributes set, then adds the association between the log group(s) and the
// service/environment names to the entitystore extension.
type awsEntityProcessor struct {
	logger *zap.Logger
}

func newAwsEntityProcessor(logger *zap.Logger) *awsEntityProcessor {
	return &awsEntityProcessor{
		logger: logger,
	}
}

func (p *awsEntityProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		resourceAttrs := rm.At(i).Resource().Attributes()
		logGroupNames, _ := resourceAttrs.Get(attributeAwsLogGroupNames)
		serviceName, _ := resourceAttrs.Get(attributeServiceName)
		environmentName, _ := resourceAttrs.Get(attributeDeploymentEnvironment)

		if logGroupNames.Str() == "" || (serviceName.Str() == "" && environmentName.Str() == "") {
			continue
		}

		logGroupNamesSlice := strings.Split(logGroupNames.Str(), "&")
		for _, logGroupName := range logGroupNamesSlice {
			if logGroupName == "" {
				continue
			}
			addToEntityStore(entitystore.LogGroupName(logGroupName), serviceName.Str(), environmentName.Str())
		}
	}

	return md, nil
}
