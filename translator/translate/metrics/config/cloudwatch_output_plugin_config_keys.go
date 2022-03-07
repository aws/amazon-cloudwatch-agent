// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

// CloudWatchOutputPluginKeys This served as a set that contains the supported CloudWatch output plugin keys
var CloudWatchOutputPluginKeys = map[string]struct{}{
	"metric_decoration":     {},
	"drop_original_metrics": {},
}

func ContainsKey(key string) bool {
	_, ok := CloudWatchOutputPluginKeys[key]
	return ok
}
