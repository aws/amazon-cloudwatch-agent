// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"go.opentelemetry.io/collector/confmap"
)

// allowedDNSSuffixes contains AWS partition DNS suffixes (e.g., amazonaws.com, api.aws).
var allowedDNSSuffixes = buildAllowedDNSSuffixes()

func buildAllowedDNSSuffixes() []string {
	var suffixes []string
	for _, p := range endpoints.DefaultPartitions() {
		suffixes = append(suffixes, p.DNSSuffix())
	}
	suffixes = append(suffixes, "api.aws")
	return suffixes
}

// otlphttpValidator is a confmap.Converter that validates otlphttp exporter endpoints
// are restricted to AWS endpoints only.
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
		if name != "otlphttp" && !strings.HasPrefix(name, "otlphttp/") {
			continue
		}
		exporterCfg, ok := cfg.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"endpoint", "metrics_endpoint", "traces_endpoint", "logs_endpoint"} {
			if ep, ok := exporterCfg[key].(string); ok && ep != "" {
				if !isAWSEndpoint(ep) {
					return fmt.Errorf("invalid AWS endpoint: %q", ep)
				}
			}
		}
	}
	return nil
}

// isAWSEndpoint checks if the endpoint host ends with an AWS DNS suffix.
func isAWSEndpoint(endpoint string) bool {
	// If endpoint doesn't contain a scheme, add https:// as default to allow for validation of domain name
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	for _, suffix := range allowedDNSSuffixes {
		if strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}
