// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/middleware"
)

const (
	refreshBuffer      = 5 * time.Minute
	maxJitter          = 5 * time.Minute
	maxStartupJitter   = 10 * time.Second
	minRefreshInterval = 1 * time.Minute
)

// Extension implements the OTEL extension interface and provides AWS credentials
// obtained by exchanging cloud provider OIDC tokens via STS AssumeRoleWithWebIdentity.
//
// It implements aws.CredentialsProvider and prepends itself into the agent's
// credential chain on Start(). All credentials stay in memory — no temp files
// or environment variable mutation.
type Extension struct {
	logger   *zap.Logger
	config   *Config
	provider TokenProvider

	mu          sync.RWMutex
	credentials *ststypes.Credentials
	done        chan struct{}
}

var (
	_ extension.Extension     = (*Extension)(nil)
	_ aws.CredentialsProvider = (*Extension)(nil)

	instance *Extension
	instMu   sync.RWMutex
)

// GetExtension returns the active cloud auth extension, or nil if not configured.
func GetExtension() *Extension {
	instMu.RLock()
	defer instMu.RUnlock()
	return instance
}

// Retrieve implements aws.CredentialsProvider. The agent's credential chain
// calls this to get the OIDC-derived AWS credentials.
func (e *Extension) Retrieve(_ context.Context) (aws.Credentials, error) {
	creds := e.getCredentials()
	if creds == nil {
		return aws.Credentials{}, fmt.Errorf("cloudauth: no credentials available")
	}
	return aws.Credentials{
		AccessKeyID:     aws.ToString(creds.AccessKeyId),
		SecretAccessKey: aws.ToString(creds.SecretAccessKey),
		SessionToken:    aws.ToString(creds.SessionToken),
		CanExpire:       true,
		Expires:         aws.ToTime(creds.Expiration),
		Source:          "cloudauth/oidc",
	}, nil
}

func (e *Extension) Start(ctx context.Context, _ component.Host) error {
	provider, err := DetectProvider(ctx, e.config.TokenFile)
	if err != nil {
		return fmt.Errorf("cloudauth: %w", err)
	}
	e.provider = provider
	e.logger.Info("Cloud auth provider detected", zap.String("provider", provider.Name()))

	// Apply custom STS resource if configured.
	if e.config.STSResource != "" {
		if ap, ok := provider.(*AzureProvider); ok {
			ap.resource = e.config.STSResource
		}
	}

	// Jitter the initial STS call to spread fleet-wide restarts.
	startupJitter := hostJitter(maxStartupJitter)
	e.logger.Info("Waiting before initial credential fetch", zap.Duration("jitter", startupJitter))
	time.Sleep(startupJitter)

	if err := e.refresh(ctx); err != nil {
		return fmt.Errorf("cloudauth: initial credential fetch failed: %w", err)
	}

	// Prepend ourselves into the credential chain so all SDK clients pick up
	// the OIDC-derived credentials automatically.
	chain := configaws.CredentialsChain()
	cloudauthProvider := configaws.CredentialsProvider{
		Name: func() string { return "CloudAuthOIDCProvider" },
		Provider: func(cc *configaws.CredentialsConfig) aws.CredentialsProvider {
			// Signal that role assumption is already done via OIDC so
			// LoadConfig doesn't wrap us with a second sts:AssumeRole.
			if cc != nil {
				cc.SetRoleAssumed(true)
			}
			return aws.NewCredentialsCache(e)
		},
	}
	configaws.OverwriteCredentialsChain(append([]configaws.CredentialsProvider{cloudauthProvider}, chain...)...)

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

	instMu.Lock()
	if instance == e {
		instance = nil
	}
	instMu.Unlock()

	return nil
}

func (e *Extension) refreshLoop() {
	for {
		interval := e.nextRefreshInterval()
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := e.refresh(ctx); err != nil {
				e.logger.Error("Credential refresh failed, will retry",
					zap.Error(err),
					zap.Duration("retry_in", minRefreshInterval))
				cancel()
				retryTimer := time.NewTimer(minRefreshInterval)
				select {
				case <-retryTimer.C:
				case <-e.done:
					retryTimer.Stop()
					return
				}
				continue
			}
			cancel()
			e.logger.Info("Credentials refreshed successfully",
				zap.String("provider", e.provider.Name()),
				zap.Time("expiration", *e.getCredentials().Expiration))
		case <-e.done:
			timer.Stop()
			return
		}
	}
}

func (e *Extension) refresh(ctx context.Context) error {
	token, _, err := e.provider.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("get OIDC token from %s: %w", e.provider.Name(), err)
	}

	stsCreds, err := e.assumeRoleWithWebIdentity(ctx, token)
	if err != nil {
		return fmt.Errorf("STS AssumeRoleWithWebIdentity: %w", err)
	}

	e.mu.Lock()
	e.credentials = stsCreds
	e.mu.Unlock()

	return nil
}

func (e *Extension) assumeRoleWithWebIdentity(ctx context.Context, token string) (*ststypes.Credentials, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(e.config.Region),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
		config.WithClientLogMode(configaws.SDKLogLevel()),
		config.WithLogger(configaws.SDKLogger{}),
	)
	if err != nil {
		return nil, fmt.Errorf("create STS config: %w", err)
	}

	var stsOpts []func(*sts.Options)

	sourceAccount := os.Getenv(envconfig.AmzSourceAccount)
	sourceArn := os.Getenv(envconfig.AmzSourceArn)
	if sourceAccount != "" && sourceArn != "" {
		stsOpts = append(stsOpts, func(o *sts.Options) {
			o.APIOptions = append(o.APIOptions, func(s *smithymiddleware.Stack) error {
				return s.Build.Add(middleware.NewCustomHeaderMiddleware("CloudAuthConfusedDeputy", map[string]string{
					"x-amz-source-arn":     sourceArn,
					"x-amz-source-account": sourceAccount,
				}), smithymiddleware.Before)
			})
		})
	}

	stsClient := sts.NewFromConfig(cfg, stsOpts...)

	result, err := stsClient.AssumeRoleWithWebIdentity(ctx, &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(e.config.RoleARN),
		RoleSessionName:  aws.String("cloudwatch-agent-cloudauth"),
		WebIdentityToken: aws.String(token),
		DurationSeconds:  aws.Int32(3600),
	})
	if err != nil {
		return nil, err
	}
	return result.Credentials, nil
}

func (e *Extension) getCredentials() *ststypes.Credentials {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.credentials
}

func (e *Extension) nextRefreshInterval() time.Duration {
	creds := e.getCredentials()
	if creds == nil || creds.Expiration == nil {
		return minRefreshInterval
	}
	until := time.Until(*creds.Expiration) - refreshBuffer - hostJitter(maxJitter)
	if until < minRefreshInterval {
		return minRefreshInterval
	}
	return until
}

// hostJitter returns a deterministic jitter duration based on the hostname.
// All agents on the same host get the same offset, but different hosts spread
// their refresh calls across the jitter window to avoid thundering herd on STS.
func hostJitter(maxDuration time.Duration) time.Duration {
	hostName, _ := os.Hostname()
	h := fnv.New64()
	h.Write([]byte(hostName))
	// Right shift by one to ensure the value fits in int64 (max 2^63-1).
	return time.Duration(int64(h.Sum64()>>1)) % maxDuration //nolint:gosec
}

// IsActive returns true if the extension has valid credentials.
func (e *Extension) IsActive() bool {
	return e.getCredentials() != nil
}
