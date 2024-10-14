// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
)

func GetSettings(uris []string, logger *zap.Logger) otelcol.ConfigProviderSettings {
	settings := otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs: uris,
			ProviderFactories: []confmap.ProviderFactory{
				fileprovider.NewFactory(),
				envprovider.NewFactory(),
			},
			ProviderSettings:   confmap.ProviderSettings{Logger: logger},
			ConverterFactories: []confmap.ConverterFactory{expandconverter.NewFactory()},
			ConverterSettings:  confmap.ConverterSettings{Logger: logger},
		},
	}
	return settings
}
