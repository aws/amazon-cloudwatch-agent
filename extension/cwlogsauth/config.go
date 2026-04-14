// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cwlogsauth

import "go.opentelemetry.io/collector/component"

// Config for the cwlogsauth extension.
type Config struct {
	// Auth is the ID of an auth extension (e.g., sigv4auth) to chain with.
	// The cwlogsauth extension wraps this auth's RoundTripper to add
	// lazy log group/stream creation before forwarding requests.
	Auth *component.ID `mapstructure:"auth,omitempty"`
}
