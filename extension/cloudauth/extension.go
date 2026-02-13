// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

const (
	refreshBuffer      = 5 * time.Minute
	minRefreshInterval = 1 * time.Minute
	tokenFilePerms     = 0600
)

// Extension implements the OTEL extension interface and provides OIDC token
// management for AWS authentication. It writes tokens to a file and sets
// environment variables for the AWS SDK to use natively via
// AssumeRoleWithWebIdentity.
type Extension struct {
	logger     *zap.Logger
	config     *Config
	provider   TokenProvider
	tokenFile  string
	done       chan struct{}
	mu         sync.RWMutex
	lastExpiry time.Time
}

var (
	_ extension.Extension = (*Extension)(nil)

	instance *Extension
	instMu   sync.RWMutex
)

// GetExtension returns the active cloud auth extension, or nil if not configured.
func GetExtension() *Extension {
	instMu.RLock()
	defer instMu.RUnlock()
	return instance
}

func (e *Extension) Start(ctx context.Context, _ component.Host) error {
	provider, err := DetectProvider(ctx, e.config.TokenFile)
	if err != nil {
		return fmt.Errorf("cloudauth: %w", err)
	}
	e.provider = provider
	e.logger.Info("Cloud auth provider detected", zap.String("provider", provider.Name()))

	// Apply custom STS resource if configured
	if e.config.STSResource != "" {
		if ap, ok := provider.(*AzureProvider); ok {
			ap.resource = e.config.STSResource
		}
	}

	// Create token directory and file
	tokenDir := filepath.Join(paths.AgentDir, "var")
	if err := os.MkdirAll(tokenDir, 0755); err != nil {
		return fmt.Errorf("cloudauth: failed to create token directory: %w", err)
	}
	e.tokenFile = filepath.Join(tokenDir, "cloudauth-token")

	// Set environment variables for AWS SDK
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", e.tokenFile)
	os.Setenv("AWS_ROLE_ARN", e.config.RoleARN)
	os.Setenv("AWS_ROLE_SESSION_NAME", "cloudwatch-agent-cloudauth")
	if e.config.Region != "" {
		os.Setenv("AWS_REGION", e.config.Region)
	}

	// Initial token fetch and write
	if err := e.refreshToken(ctx); err != nil {
		return fmt.Errorf("cloudauth: initial token fetch failed: %w", err)
	}

	e.done = make(chan struct{})
	go e.refreshLoop()

	instMu.Lock()
	instance = e
	instMu.Unlock()

	return nil
}

func (e *Extension) Shutdown(_ context.Context) error {
	if e.done != nil {
		close(e.done)
	}

	// Clean up token file
	if e.tokenFile != "" {
		os.Remove(e.tokenFile)
	}

	instMu.Lock()
	if instance == e {
		instance = nil
	}
	instMu.Unlock()

	return nil
}

func (e *Extension) refreshLoop() {
	for {
		e.mu.RLock()
		expiry := e.lastExpiry
		e.mu.RUnlock()

		// Calculate next refresh with buffer
		interval := time.Until(expiry) - refreshBuffer
		if interval < minRefreshInterval {
			interval = minRefreshInterval
		}

		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := e.refreshToken(ctx); err != nil {
				e.logger.Error("Token refresh failed, will retry",
					zap.Error(err),
					zap.Duration("retry_in", minRefreshInterval))
				cancel()
				time.Sleep(minRefreshInterval)
			} else {
				e.logger.Info("Token refreshed successfully",
					zap.String("provider", e.provider.Name()))
				cancel()
			}
		case <-e.done:
			timer.Stop()
			return
		}
	}
}

func (e *Extension) refreshToken(ctx context.Context) error {
	token, expiry, err := e.provider.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("get OIDC token from %s: %w", e.provider.Name(), err)
	}

	// Write token to file
	if err := os.WriteFile(e.tokenFile, []byte(token), tokenFilePerms); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}

	// Update expiry
	e.mu.Lock()
	e.lastExpiry = time.Now().Add(expiry)
	e.mu.Unlock()

	return nil
}

// IsActive returns true if the extension has a valid token.
func (e *Extension) IsActive() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return !e.lastExpiry.IsZero() && time.Now().Before(e.lastExpiry)
}
