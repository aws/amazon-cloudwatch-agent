// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"context"
	"fmt"
)

// registeredProviders is the ordered list of providers to try during auto-detection.
var registeredProviders = []func() TokenProvider{
	func() TokenProvider { return NewAzureProvider() },
}

// DetectProvider returns a TokenProvider for the current environment.
// If tokenFile is set, a FileProvider is returned directly (no probing).
// Otherwise, registered providers are probed in order.
func DetectProvider(ctx context.Context, tokenFile string) (TokenProvider, error) {
	if tokenFile != "" {
		fp := NewFileProvider(tokenFile)
		if !fp.IsAvailable(ctx) {
			return nil, fmt.Errorf("cloudauth: token file %q does not exist", tokenFile)
		}
		return fp, nil
	}

	for _, newProvider := range registeredProviders {
		p := newProvider()
		if p.IsAvailable(ctx) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("cloudauth: no OIDC provider detected in current environment")
}
