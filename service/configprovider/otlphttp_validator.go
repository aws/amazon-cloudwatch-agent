// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"go.opentelemetry.io/collector/confmap"
)

const (
	endpointKey        = "endpoint"
	metricsEndpointKey = "metrics_endpoint"
	tracesEndpointKey  = "traces_endpoint"
	logsEndpointKey    = "logs_endpoint"
)

// allowedEndpointPatterns maps endpoint config keys to their allowed URL patterns.
// Patterns are built at init time from AWS SDK partition data.
var allowedEndpointPatterns = buildAllowedEndpointPatterns()

// buildAllowedEndpointPatterns creates regex patterns for valid AWS OTLP endpoints.
// Uses DNS suffixes from SDK partitions (e.g., amazonaws.com, amazonaws.com.cn)
// combined with a region pattern that matches all AWS region naming conventions.
func buildAllowedEndpointPatterns() map[string][]*regexp.Regexp {
	patterns := map[string][]*regexp.Regexp{
		endpointKey:        {},
		metricsEndpointKey: {},
		tracesEndpointKey:  {},
		logsEndpointKey:    {},
	}
	regionPattern := `[a-z]{2}(-[a-z]+)+-\d`
	for _, p := range endpoints.DefaultPartitions() {
		suffix := regexp.QuoteMeta(p.DNSSuffix())
		// Base URL patterns for generic endpoint (no path suffix)
		for _, svc := range []string{"monitoring", "xray", "logs"} {
			patterns[endpointKey] = append(patterns[endpointKey],
				regexp.MustCompile(fmt.Sprintf(`^https://%s\.%s\.%s$`, svc, regionPattern, suffix)))
		}
		// Full path patterns for signal-specific endpoints
		patterns[metricsEndpointKey] = append(patterns[metricsEndpointKey],
			regexp.MustCompile(fmt.Sprintf(`^https://monitoring\.%s\.%s/v1/metrics$`, regionPattern, suffix)))
		patterns[tracesEndpointKey] = append(patterns[tracesEndpointKey],
			regexp.MustCompile(fmt.Sprintf(`^https://xray\.%s\.%s/v1/traces$`, regionPattern, suffix)))
		patterns[logsEndpointKey] = append(patterns[logsEndpointKey],
			regexp.MustCompile(fmt.Sprintf(`^https://logs\.%s\.%s/v1/logs$`, regionPattern, suffix)))
	}
	return patterns
}

// otlphttpValidator is a confmap.Converter that validates otlphttp exporter endpoints
// are restricted to AWS OTLP endpoints only.
type otlphttpValidator struct{}

// NewOTLPHTTPValidatorFactory returns a factory for creating otlphttp endpoint validators.
func NewOTLPHTTPValidatorFactory() confmap.ConverterFactory {
	return confmap.NewConverterFactory(func(_ confmap.ConverterSettings) confmap.Converter {
		return &otlphttpValidator{}
	})
}

// Convert validates that all otlphttp exporters use only allowed AWS endpoints.
func (v *otlphttpValidator) Convert(_ context.Context, conf *confmap.Conf) error {
	exportersVal := conf.Get("exporters")
	if exportersVal == nil {
		return nil
	}
	exporters, ok := exportersVal.(map[string]any)
	if !ok {
		return nil
	}
	for name, cfg := range exporters {
		if !strings.HasPrefix(name, "otlphttp") {
			continue
		}
		exporterCfg, ok := cfg.(map[string]any)
		if !ok {
			continue
		}
		if err := validateEndpoints(exporterCfg); err != nil {
			return err
		}
	}
	return nil
}

// validateEndpoints checks that all endpoint URLs in the config are allowed.
func validateEndpoints(cfg map[string]any) error {
	for _, key := range []string{endpointKey, metricsEndpointKey, tracesEndpointKey, logsEndpointKey} {
		ep, ok := cfg[key].(string)
		if !ok || ep == "" {
			continue
		}
		if !matchesAny(ep, allowedEndpointPatterns[key]) {
			return fmt.Errorf("the cloudwatch agent does not support 3rd party exportation of its telemetry; endpoint %q is not allowed, use only the allowlisted endpoints", ep)
		}
	}
	return nil
}

func matchesAny(s string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}
