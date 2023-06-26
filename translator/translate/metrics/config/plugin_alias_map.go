// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

// pluginAliasMap This provides the real plugin name mapping to the measurement name in user config
var pluginAliasMap = map[string]string{
	"nvidia_gpu": "nvidia_smi",
}

func GetRealPluginName(inputPluginName string) string {
	if result, ok := pluginAliasMap[inputPluginName]; ok {
		return result
	}
	// if there is not such mapping, the plugin do not use an alias in config
	return inputPluginName
}
