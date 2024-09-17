// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/s3provider"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/confmap/provider/httpprovider"
	"go.opentelemetry.io/collector/confmap/provider/httpsprovider"
	"go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
)

func GetSettings(configPath string, logger *zap.Logger) otelcol.ConfigProviderSettings {
	settings := otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs: []string{configPath},
			ProviderFactories: []confmap.ProviderFactory{
				fileprovider.NewFactory(),
				envprovider.NewFactory(),
				yamlprovider.NewFactory(),
				httpprovider.NewFactory(),
				httpsprovider.NewFactory(),
				s3provider.NewFactory(),
			},
			ProviderSettings:   confmap.ProviderSettings{Logger: logger},
			ConverterFactories: []confmap.ConverterFactory{expandconverter.NewFactory()},
			ConverterSettings:  confmap.ConverterSettings{Logger: logger},
		},
	}
	return settings
}
