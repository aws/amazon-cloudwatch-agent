// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"go.opentelemetry.io/collector/component"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
)

// pipelineDefaults holds the common OTLP exporter and SigV4 extension
// shared by all pipelines under opentelemetry.collect.
type pipelineDefaults struct {
	Endpoint otlphttp.EndpointConfig
	AuthID   component.ID
	SigV4Ext common.ComponentTranslator
}

func newPipelineDefaults(pipelineName string) (*pipelineDefaults, error) {
	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s pipeline", pipelineName)
	}
	metricsEndpoint := serviceEndpoint("monitoring", region, "/v1/metrics")
	sigv4Ext := sigv4auth.NewTranslatorWithService("monitoring")
	return &pipelineDefaults{
		Endpoint: otlphttp.EndpointConfig{MetricsEndpoint: metricsEndpoint},
		AuthID:   sigv4Ext.ID(),
		SigV4Ext: sigv4Ext,
	}, nil
}

func serviceEndpoint(service, region, path string) string {
	partition, _ := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	dnsSuffix := partition.DNSSuffix()
	if dnsSuffix == "" {
		dnsSuffix = "amazonaws.com"
	}
	return fmt.Sprintf("https://%s.%s.%s%s", service, region, dnsSuffix, path)
}
