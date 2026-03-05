// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/merge/confmap"
	agenthealthtranslator "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

const (
	featureFlagOtelMergeYAML = "otel_merge_yaml"
	featureFlagOtelMergeJSON = "otel_merge_json"
)

// mergeConfigs merges multiple OTEL configs together, including any config
// provided via the CW_CONFIG_CONTENT environment variable when running in a
// container. Returns nil without an error if there is nothing to merge (i.e.
// a single config path with no env override). In practice, a single config
// means no custom YAML was provided — the default agent YAML is always
// accompanied by at least one custom YAML when custom configs are in use.
func mergeConfigs(configPaths []string, isUsageDataEnabled bool) (*confmap.Conf, error) {
	var loaders []confmap.Loader
	if envconfig.IsRunningInContainer() {
		content, ok := os.LookupEnv(envconfig.CWOtelConfigContent)
		if ok && len(content) > 0 {
			log.Printf("D! Merging OTEL configuration from: %s", envconfig.CWOtelConfigContent)
			loaders = append(loaders, confmap.NewByteLoader(envconfig.CWOtelConfigContent, []byte(content)))
		}
	}
	// If using environment variable or passing in more than one config
	if len(loaders) > 0 || len(configPaths) > 1 {
		log.Printf("D! Merging OTEL configurations from: %s", configPaths)
		for _, configPath := range configPaths {
			loaders = append(loaders, confmap.NewFileLoader(configPath))
		}
		var result *confmap.Conf
		for _, loader := range loaders {
			conf, err := loader.Load()
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					log.Printf("D! Skipping non-existent OTEL config: %s", loader.ID())
					continue
				}
				return nil, fmt.Errorf("failed to load OTEL configs: %w", err)
			}
			if result == nil {
				result = confmap.New()
			}
			if err = result.Merge(conf); err != nil {
				return nil, fmt.Errorf("failed to merge OTEL configs: %w", err)
			}
		}
		return mergeAgentHealth(result, isUsageDataEnabled), nil
	}
	return nil, nil
}

type exporterInfo struct {
	middlewareID string
	operations   []any
}

var logsExporterInfo = exporterInfo{middlewareID: agenthealthtranslator.LogsID.String(), operations: []any{agenthealthtranslator.OperationPutLogEvents}}

// supportedExporters maps exporter type names to their agenthealth middleware ID and operations.
var supportedExporters = map[string]exporterInfo{
	"awscloudwatch":     {middlewareID: agenthealthtranslator.MetricsID.String(), operations: []any{agenthealthtranslator.OperationPutMetricData}},
	"awsemf":            logsExporterInfo,
	"awscloudwatchlogs": logsExporterInfo,
	"awsxray":           {middlewareID: agenthealthtranslator.TracesID.String(), operations: []any{agenthealthtranslator.OperationPutTraceSegments}},
}

// mergeAgentHealth scans the exporters in the config for supported AWS exporters
// and adds the appropriate agenthealth extension with a middleware reference to each.
func mergeAgentHealth(conf *confmap.Conf, isUsageDataEnabled bool) *confmap.Conf {
	if conf == nil || !isUsageDataEnabled {
		return conf
	}

	cfgMap := conf.ToStringMap()

	exporters, ok := cfgMap["exporters"].(map[string]any)
	if !ok {
		return conf
	}

	// Track which agenthealth extensions are needed
	neededExtensions := make(map[string]exporterInfo)
	for key := range exporters {
		typeName, _, _ := strings.Cut(key, "/")
		info, ok := supportedExporters[typeName]
		if !ok {
			continue
		}
		exporterCfg, ok := exporters[key].(map[string]any)
		if !ok || exporterCfg == nil {
			exporterCfg = make(map[string]any)
			exporters[key] = exporterCfg
		}
		if _, alreadySet := exporterCfg["middleware"]; !alreadySet {
			exporterCfg["middleware"] = info.middlewareID
			neededExtensions[info.middlewareID] = info
		}
	}

	if len(neededExtensions) == 0 {
		return conf
	}

	// Ensure extensions section exists
	extensions, _ := cfgMap["extensions"].(map[string]any)
	if extensions == nil {
		extensions = make(map[string]any)
		cfgMap["extensions"] = extensions
	}

	// Ensure service section exists
	service, _ := cfgMap["service"].(map[string]any)
	if service == nil {
		service = make(map[string]any)
		cfgMap["service"] = service
	}

	var svcExtensions []any
	if existing, ok := service["extensions"].([]any); ok {
		svcExtensions = existing
	}

	for middlewareID, info := range neededExtensions {
		if _, exists := extensions[middlewareID]; !exists {
			extensions[middlewareID] = map[string]any{
				"is_usage_data_enabled": true,
				"stats": map[string]any{
					"operations": info.operations,
				},
			}
		}
		if !slices.Contains(svcExtensions, any(middlewareID)) {
			svcExtensions = append(svcExtensions, middlewareID)
		}
	}

	service["extensions"] = svcExtensions
	return confmap.NewFromStringMap(cfgMap)
}
