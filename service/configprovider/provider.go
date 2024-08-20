// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
)

func GetSettings(configPath string, logger *zap.Logger) otelcol.ConfigProviderSettings {
	settings := otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs:               []string{configPath},
			ProviderFactories:  []confmap.ProviderFactory{fileprovider.NewFactory()},
			ProviderSettings:   confmap.ProviderSettings{Logger: logger},
			ConverterFactories: []confmap.ConverterFactory{expandconverter.NewFactory()},
			ConverterSettings:  confmap.ConverterSettings{Logger: logger},
		},
	}
	return settings
}
