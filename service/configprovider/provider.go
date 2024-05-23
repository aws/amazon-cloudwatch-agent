// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
)

func Get(configPath string) (otelcol.ConfigProvider, error) {
	fprovider := fileprovider.NewWithSettings(confmap.ProviderSettings{})
	settings := otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs:       []string{configPath},
			Converters: []confmap.Converter{expandconverter.New(confmap.ConverterSettings{})},
			Providers: map[string]confmap.Provider{
				fprovider.Scheme(): fprovider,
			},
		},
	}
	return otelcol.NewConfigProvider(settings)
}
