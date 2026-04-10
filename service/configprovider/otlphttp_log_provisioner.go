// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"go.opentelemetry.io/collector/confmap"
)

// cwLogsEndpointPattern matches CloudWatch Logs OTLP endpoints like
// https://logs.us-east-1.amazonaws.com/v1/logs
var cwLogsEndpointPattern = regexp.MustCompile(`^https://logs\.([a-z0-9-]+)\.amazonaws\.com`)

const (
	headerLogGroup  = "x-aws-log-group"
	headerLogStream = "x-aws-log-stream"
)

// otlphttpLogProvisioner is a confmap.Converter that auto-creates CloudWatch
// log groups and log streams for otlphttp exporters targeting CW Logs OTLP endpoints.
// It runs once at startup before the OTel collector starts.
type otlphttpLogProvisioner struct{}

// NewOTLPHTTPLogProvisionerFactory returns a factory for creating the log provisioner.
func NewOTLPHTTPLogProvisionerFactory() confmap.ConverterFactory {
	return confmap.NewConverterFactory(func(_ confmap.ConverterSettings) confmap.Converter {
		return &otlphttpLogProvisioner{}
	})
}

// logTarget represents a (log group, log stream, region) tuple to be provisioned.
type logTarget struct {
	logGroupName  string
	logStreamName string
	region        string
}

// Convert scans the config for otlphttp exporters used in logs pipelines that target
// CW Logs OTLP endpoints, and auto-creates the log groups and log streams.
func (p *otlphttpLogProvisioner) Convert(_ context.Context, conf *confmap.Conf) error {
	targets := p.findLogTargets(conf)
	if len(targets) == 0 {
		return nil
	}

	for _, t := range targets {
		if err := provisionLogGroupAndStream(t); err != nil {
			// Log warning but don't block startup
			log.Printf("W! Failed to auto-create log group/stream (group=%s, stream=%s, region=%s): %v",
				t.logGroupName, t.logStreamName, t.region, err)
		} else {
			log.Printf("I! Auto-provisioned log group/stream (group=%s, stream=%s, region=%s)",
				t.logGroupName, t.logStreamName, t.region)
		}
	}

	// Never return error — provisioning failures should not block CWAgent startup
	return nil
}

// findLogTargets identifies otlphttp exporters in logs pipelines that target CW Logs
// OTLP endpoints, and extracts the log group/stream from their headers.
func (p *otlphttpLogProvisioner) findLogTargets(conf *confmap.Conf) []logTarget {
	// 1. Get all exporters used in logs/* pipelines
	logsExporterNames := p.getLogsExporterNames(conf)
	if len(logsExporterNames) == 0 {
		return nil
	}

	// 2. Get exporter configs
	exportersVal := conf.Get("exporters")
	if exportersVal == nil {
		return nil
	}
	exporters, ok := exportersVal.(map[string]any)
	if !ok {
		return nil
	}

	var targets []logTarget

	for name, cfg := range exporters {
		// Only otlphttp exporters
		if name != "otlphttp" && !strings.HasPrefix(name, "otlphttp/") {
			continue
		}
		// Only exporters used in logs pipelines
		if !logsExporterNames[name] {
			continue
		}

		exporterCfg, ok := cfg.(map[string]any)
		if !ok {
			continue
		}

		// Check if logs_endpoint or endpoint targets CW Logs
		endpoint := ""
		if ep, ok := exporterCfg["logs_endpoint"].(string); ok && ep != "" {
			endpoint = ep
		} else if ep, ok := exporterCfg["endpoint"].(string); ok && ep != "" {
			endpoint = ep
		}

		region := extractRegionFromLogsEndpoint(endpoint)
		if region == "" {
			continue // Not a CW Logs endpoint
		}

		// Extract log group and stream from headers
		headers, ok := exporterCfg["headers"].(map[string]any)
		if !ok {
			continue
		}

		logGroup, _ := headers[headerLogGroup].(string)
		logStream, _ := headers[headerLogStream].(string)

		if logGroup == "" {
			continue // No log group configured
		}
		if logStream == "" {
			logStream = "default"
		}

		targets = append(targets, logTarget{
			logGroupName:  logGroup,
			logStreamName: logStream,
			region:        region,
		})
	}

	return targets
}

// getLogsExporterNames returns the set of exporter names used in logs/* pipelines.
func (p *otlphttpLogProvisioner) getLogsExporterNames(conf *confmap.Conf) map[string]bool {
	result := make(map[string]bool)

	serviceVal := conf.Get("service")
	if serviceVal == nil {
		return result
	}
	service, ok := serviceVal.(map[string]any)
	if !ok {
		return result
	}
	pipelines, ok := service["pipelines"].(map[string]any)
	if !ok {
		return result
	}

	for pipelineName, pipelineCfg := range pipelines {
		if !strings.HasPrefix(pipelineName, "logs") {
			continue
		}
		pipeline, ok := pipelineCfg.(map[string]any)
		if !ok {
			continue
		}
		exporters, ok := pipeline["exporters"].([]any)
		if !ok {
			continue
		}
		for _, exp := range exporters {
			if name, ok := exp.(string); ok {
				result[name] = true
			}
		}
	}

	return result
}

// extractRegionFromLogsEndpoint extracts the AWS region from a CW Logs OTLP endpoint.
// Returns empty string if the endpoint is not a CW Logs endpoint.
func extractRegionFromLogsEndpoint(endpoint string) string {
	matches := cwLogsEndpointPattern.FindStringSubmatch(endpoint)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// provisionLogGroupAndStream creates the log group and log stream if they don't exist.
// Follows the same pattern as cwlogs.Client.CreateStream:
// 1. Try CreateLogStream
// 2. If ResourceNotFoundException -> CreateLogGroup -> retry CreateLogStream
// 3. If ResourceAlreadyExistsException -> ignore (idempotent)
func provisionLogGroupAndStream(t logTarget) error {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(t.region),
	})
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}

	svc := cloudwatchlogs.New(sess)
	logGroup := aws.String(t.logGroupName)
	logStream := aws.String(t.logStreamName)

	// Try creating the log stream first
	_, err = svc.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  logGroup,
		LogStreamName: logStream,
	})
	if err == nil {
		return nil
	}

	awsErr, ok := err.(awserr.Error)
	if !ok {
		return err
	}

	switch awsErr.Code() {
	case cloudwatchlogs.ErrCodeResourceAlreadyExistsException:
		return nil

	case cloudwatchlogs.ErrCodeResourceNotFoundException:
		// Log group doesn't exist — create it, then retry stream creation
		_, err = svc.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
			LogGroupName: logGroup,
		})
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceAlreadyExistsException {
				// Race condition: another process created it
			} else {
				return fmt.Errorf("failed to create log group %q: %w", t.logGroupName, err)
			}
		}

		// Retry creating the log stream
		_, err = svc.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
			LogGroupName:  logGroup,
			LogStreamName: logStream,
		})
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceAlreadyExistsException {
				return nil
			}
			return fmt.Errorf("failed to create log stream %q in group %q: %w", t.logStreamName, t.logGroupName, err)
		}
		return nil

	default:
		return fmt.Errorf("unexpected error creating log stream: %w", err)
	}
}