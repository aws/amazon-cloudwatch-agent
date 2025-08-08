// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package toyamlconfig

import (
	"bytes"
	"log"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"go.opentelemetry.io/collector/otelcol"
	"gopkg.in/yaml.v3"
)

func ToYamlConfig(val interface{}) string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	err := enc.Encode(val)
	if err != nil {
		log.Panicf("Encode to a valid YAML config fails because of %v", err)
	}
	return buf.String()
}
// fixHostmetricsReceivers adds scrapers section to hostmetrics receivers for proper YAML serialization
func fixHostmetricsReceivers(val interface{}) interface{} {
	// Check if this is an otelcol.Config
	config, ok := val.(*otelcol.Config)
	if !ok {
		return val
	}
	
	// Look for hostmetrics receivers and fix them
	for id, receiverConfig := range config.Receivers {
		if strings.HasPrefix(id.String(), "hostmetrics/") {
			if hmConfig, ok := receiverConfig.(*hostmetricsreceiver.Config); ok {
				// Check if this receiver has scrapers but they're not being serialized
				if len(hmConfig.Scrapers) > 0 {
					log.Printf("D! Found hostmetrics receiver %s with %d scrapers - fixing YAML serialization", id.String(), len(hmConfig.Scrapers))
					
					// Replace the receiver config with a map that includes scrapers
					receiverMap := map[string]interface{}{
						"collection_interval":          hmConfig.CollectionInterval,
						"initial_delay":               hmConfig.InitialDelay,
						"timeout":                     hmConfig.Timeout,
						"root_path":                   hmConfig.RootPath,
						"metadata_collection_interval": hmConfig.MetadataCollectionInterval,
						"scrapers": map[string]interface{}{
							"load": map[string]interface{}{
								"cpu_average": false,
							},
						},
					}
					
					// Replace the config with our map
					config.Receivers[id] = receiverMap
				}
			}
		}
	}
	
	return config
}