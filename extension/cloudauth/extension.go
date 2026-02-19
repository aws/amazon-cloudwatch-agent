// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/cloudauth/provider"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

const (
	refreshBuffer      = 5 * time.Minute
	minRefreshInterval = 1 * time.Minute
	tokenFilePerms     = 0600
)

type Extension struct {
	logger        *zap.Logger
	config        *Config
	tokenProvider provider.TokenProvider
	tokenFile     string
	done          chan struct{}
}

var _ extension.Extension = (*Extension)(nil)

func (e *Extension) Start(ctx context.Context, _ component.Host) error {
	tp, err := DetectProvider(ctx, e.config.TokenFile)
	if err != nil {
		return fmt.Errorf("cloudauth: %w", err)
	}
	e.tokenProvider = tp
	e.logger.Info("Cloud auth provider detected", zap.String("provider", tp.Name()))

	if e.config.STSResource != "" {
		if ap, ok := tp.(*provider.AzureProvider); ok {
			ap.SetResource(e.config.STSResource)
		}
	}

	tokenDir := filepath.Join(paths.AgentDir, "var")
	if err := os.MkdirAll(tokenDir, 0755); err != nil {
		return fmt.Errorf("cloudauth: failed to create token directory: %w", err)
	}
	e.tokenFile = filepath.Join(tokenDir, "cloudauth-token")

	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", e.tokenFile)

	expiry, err := e.refreshToken(ctx)
	if err != nil {
		return fmt.Errorf("cloudauth: initial token fetch failed: %w", err)
	}

	e.done = make(chan struct{})
	go e.refreshLoop(expiry)

	return nil
}

func (e *Extension) Shutdown(_ context.Context) error {
	if e.done != nil {
		close(e.done)
	}
	if e.tokenFile != "" {
		os.Remove(e.tokenFile)
	}
	return nil
}

func (e *Extension) refreshLoop(expiry time.Time) {
	for {
		interval := time.Until(expiry) - refreshBuffer
		if interval < minRefreshInterval {
			interval = minRefreshInterval
		}

		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			newExpiry, err := e.refreshToken(ctx)
			cancel()
			if err != nil {
				e.logger.Error("Token refresh failed, will retry",
					zap.Error(err),
					zap.Duration("retry_in", minRefreshInterval))
				time.Sleep(minRefreshInterval)
			} else {
				expiry = newExpiry
				e.logger.Info("Token refreshed successfully",
					zap.String("provider", e.tokenProvider.Name()))
			}
		case <-e.done:
			timer.Stop()
			return
		}
	}
}

func (e *Extension) refreshToken(ctx context.Context) (time.Time, error) {
	token, ttl, err := e.tokenProvider.GetToken(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("get OIDC token from %s: %w", e.tokenProvider.Name(), err)
	}
	if err := os.WriteFile(e.tokenFile, []byte(token), tokenFilePerms); err != nil {
		return time.Time{}, fmt.Errorf("write token file: %w", err)
	}
	return time.Now().Add(ttl), nil
}
