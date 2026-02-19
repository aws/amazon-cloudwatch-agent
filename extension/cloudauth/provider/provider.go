// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"time"
)

// TokenProvider abstracts fetching an OIDC token from a cloud provider.
// Implementations must be safe for concurrent use.
type TokenProvider interface {
	// GetToken returns a raw OIDC/JWT token suitable for STS AssumeRoleWithWebIdentity.
	GetToken(ctx context.Context) (token string, expiry time.Duration, err error)

	// IsAvailable probes whether this provider can operate in the current environment.
	// Implementations should be lightweight (e.g. a single HTTP probe or env var check).
	IsAvailable(ctx context.Context) bool

	// Name returns a human-readable identifier for logging.
	Name() string
}
