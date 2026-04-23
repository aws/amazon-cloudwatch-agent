// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package applicationsignalslogs

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscloudwatchlogsprovisionerextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributestocontextprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
)

const (
	// Metadata keys used to pass log group/stream from attributestocontext
	// processor to the provisioner extension.
	metadataKeyLogGroup  = "cwlogs.log_group"
	metadataKeyLogStream = "cwlogs.log_stream"
)

// --- sigv4auth extension translator ---

type sigV4AuthTranslator struct{}

func newSigV4AuthTranslator() common.ComponentTranslator {
	return &sigV4AuthTranslator{}
}

func (t *sigV4AuthTranslator) ID() component.ID {
	return component.NewIDWithName(component.MustNewType("sigv4auth"), "appsignals_logs")
}

func (t *sigV4AuthTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := sigv4authextension.NewFactory().CreateDefaultConfig().(*sigv4authextension.Config)
	cfg.Region = agent.Global_Config.Region
	cfg.Service = "logs"
	if agent.Global_Config.Role_arn != "" {
		cfg.AssumeRole = sigv4authextension.AssumeRole{
			ARN:       agent.Global_Config.Role_arn,
			STSRegion: agent.Global_Config.Region,
		}
	}
	return cfg, nil
}

// --- awscloudwatchlogsprovisioner extension translator ---

type provisionerTranslator struct{}

func newProvisionerTranslator() common.ComponentTranslator {
	return &provisionerTranslator{}
}

func (t *provisionerTranslator) ID() component.ID {
	return component.MustNewID("awscloudwatchlogsprovisioner")
}

func (t *provisionerTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	sigv4AuthID := component.NewIDWithName(component.MustNewType("sigv4auth"), "appsignals_logs")
	cfg := awscloudwatchlogsprovisionerextension.NewFactory().CreateDefaultConfig().(*awscloudwatchlogsprovisionerextension.Config)
	cfg.Region = agent.Global_Config.Region
	cfg.AdditionalAuth = &sigv4AuthID
	return cfg, nil
}

// --- transform processor translator ---
// Builds full log group/stream names into resource attributes using OTTL Concat.
// Supports arbitrary placeholders: "/a/{service.name}/b/{attr}" generates
// Concat(["/a/", resource.attributes["service.name"], "/b/", resource.attributes["attr"]], "")

type transformTranslator struct {
	logGroupTemplate  []templateSegment
	logStreamTemplate []templateSegment
}

func newTransformTranslator(logGroupTemplate, logStreamTemplate []templateSegment) common.ComponentTranslator {
	return &transformTranslator{logGroupTemplate: logGroupTemplate, logStreamTemplate: logStreamTemplate}
}

func (t *transformTranslator) ID() component.ID {
	return component.NewIDWithName(component.MustNewType("transform"), pipelineName)
}

func (t *transformTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	groupStmt := buildOTTLSetStatement(metadataKeyLogGroup, t.logGroupTemplate)
	streamStmt := buildOTTLSetStatement(metadataKeyLogStream, t.logStreamTemplate)

	cfgMap := map[string]interface{}{
		"log_statements": []interface{}{
			map[string]interface{}{
				"context": "resource",
				"statements": []interface{}{
					groupStmt,
					streamStmt,
				},
			},
		},
	}

	cfg := transformprocessor.NewFactory().CreateDefaultConfig()
	if err := confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to configure transform processor: %w", err)
	}
	return cfg, nil
}

// buildOTTLSetStatement generates an OTTL statement from template segments.
// A `where` guard ensures the attribute is only set when not already present,
// so SDK-set values take precedence over the template.
func buildOTTLSetStatement(metadataKey string, segments []templateSegment) string {
	whereGuard := fmt.Sprintf(` where resource.attributes["%s"] == nil`, metadataKey)

	hasAttributes := false
	for _, seg := range segments {
		if seg.attribute != "" {
			hasAttributes = true
			break
		}
	}

	if !hasAttributes {
		var literal string
		for _, seg := range segments {
			literal += seg.literal
		}
		return fmt.Sprintf(`set(resource.attributes["%s"], "%s")`, metadataKey, literal) + whereGuard
	}

	var parts []string
	for _, seg := range segments {
		if seg.attribute != "" {
			parts = append(parts, fmt.Sprintf(`resource.attributes["%s"]`, seg.attribute))
		} else {
			parts = append(parts, fmt.Sprintf(`"%s"`, seg.literal))
		}
	}
	return fmt.Sprintf(`set(resource.attributes["%s"], Concat([%s], ""))`, metadataKey, strings.Join(parts, ", ")) + whereGuard
}

// --- attributestocontext processor translator ---
// Copies cwlogs.log_group and cwlogs.log_stream from resource attributes to
// client.Metadata, making them available to the provisioner extension.

type attributesToContextTranslator struct{}

func newAttributesToContextTranslator() common.ComponentTranslator {
	return &attributesToContextTranslator{}
}

func (t *attributesToContextTranslator) ID() component.ID {
	return component.MustNewID("attributestocontext")
}

func (t *attributesToContextTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := attributestocontextprocessor.NewFactory().CreateDefaultConfig()
	cfgMap := map[string]interface{}{
		"actions": []interface{}{
			map[string]interface{}{
				"key":                     metadataKeyLogGroup,
				"from_resource_attribute": metadataKeyLogGroup,
			},
			map[string]interface{}{
				"key":                     metadataKeyLogStream,
				"from_resource_attribute": metadataKeyLogStream,
			},
		},
	}
	if err := confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to configure attributestocontext: %w", err)
	}
	return cfg, nil
}

// --- batch processor translators ---

type batchTranslator struct {
	withMetadataKeys bool
}

func newBatchWithMetadataKeysTranslator() common.ComponentTranslator {
	return &batchTranslator{withMetadataKeys: true}
}

func newBatchTranslator() common.ComponentTranslator {
	return &batchTranslator{}
}

func (t *batchTranslator) ID() component.ID {
	return component.NewIDWithName(component.MustNewType("batch"), pipelineName)
}

func (t *batchTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := batchprocessor.NewFactory().CreateDefaultConfig().(*batchprocessor.Config)
	if t.withMetadataKeys {
		cfg.MetadataKeys = []string{metadataKeyLogGroup, metadataKeyLogStream}
	}
	return cfg, nil
}

// --- headers_setter extension translator ---
// Sets x-aws-log-group and x-aws-log-stream headers from client.Metadata
// (populated by attributestocontext). Chains to the provisioner for log
// group creation before sigv4auth signs the request.

type headersSetterTranslator struct{}

func newHeadersSetterTranslator() common.ComponentTranslator {
	return &headersSetterTranslator{}
}

func (t *headersSetterTranslator) ID() component.ID {
	return component.NewIDWithName(component.MustNewType("headers_setter"), "appsignals_logs")
}

func (t *headersSetterTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	provisionerID := component.MustNewID("awscloudwatchlogsprovisioner")
	cfg := headerssetterextension.NewFactory().CreateDefaultConfig().(*headerssetterextension.Config)
	cfg.AdditionalAuth = &provisionerID

	logGroupKey := metadataKeyLogGroup
	logStreamKey := metadataKeyLogStream
	cfg.HeadersConfig = []headerssetterextension.HeaderConfig{
		{
			Key:         strPtr("x-aws-log-group"),
			FromContext: &logGroupKey,
			Action:      headerssetterextension.UPSERT,
		},
		{
			Key:         strPtr("x-aws-log-stream"),
			FromContext: &logStreamKey,
			Action:      headerssetterextension.UPSERT,
		},
	}
	return cfg, nil
}

func strPtr(s string) *string { return &s }

// --- otlphttp exporter translators ---

// newDynamicOTLPHTTPExporterTranslator creates the otlphttp exporter for the
// dynamic pipeline, authenticating through headers_setter (which chains to
// provisioner → sigv4auth).
func newDynamicOTLPHTTPExporterTranslator() common.ComponentTranslator {
	headersSetterID := component.NewIDWithName(component.MustNewType("headers_setter"), "appsignals_logs")
	return otlphttp.NewTranslatorWithName("appsignals_logs", otlphttp.WithAuthenticator(headersSetterID))
}

// newStaticOTLPHTTPExporterTranslator creates the otlphttp exporter for the
// static pipeline with hardcoded x-aws-log-group/stream headers, authenticating
// directly through the provisioner (which chains to sigv4auth).
func newStaticOTLPHTTPExporterTranslator(headers map[string]string) common.ComponentTranslator {
	provisionerID := component.MustNewID("awscloudwatchlogsprovisioner")
	return otlphttp.NewTranslatorWithName("appsignals_logs",
		otlphttp.WithAuthenticator(provisionerID),
		otlphttp.WithHeaders(headers),
	)
}
