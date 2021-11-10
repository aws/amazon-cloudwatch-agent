// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

// TagDenyList This served as the denylist tag name, which is registered under the plugin name
var TagDenyList = map[string][]string{
	"nvidia_smi": {"compute_mode", "pstate", "uuid"},
}
