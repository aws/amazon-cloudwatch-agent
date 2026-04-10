// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cwlogsprovision

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.uber.org/zap"
)

const (
	headerLogGroup  = "X-Aws-Log-Group"
	headerLogStream = "X-Aws-Log-Stream"
)

type cwLogsProvisionExtension struct {
	logger *zap.Logger
	cfg    *Config
	host   component.Host
	component.ShutdownFunc

	// Track which (group, stream) pairs have been provisioned
	provisioned sync.Map // key: "group\x00stream" -> bool
}

var _ extensionauth.HTTPClient = (*cwLogsProvisionExtension)(nil)
var _ extensioncapabilities.Dependent = (*cwLogsProvisionExtension)(nil)

func newExtension(logger *zap.Logger, cfg *Config) *cwLogsProvisionExtension {
	return &cwLogsProvisionExtension{
		logger: logger,
		cfg:    cfg,
	}
}

func (e *cwLogsProvisionExtension) Start(_ context.Context, host component.Host) error {
	e.host = host
	return nil
}

func (e *cwLogsProvisionExtension) Dependencies() []component.ID {
	if e.cfg.Auth == nil {
		return nil
	}
	return []component.ID{*e.cfg.Auth}
}

// RoundTripper wraps the base transport (and optional inner auth extension)
// with lazy log group/stream provisioning.
func (e *cwLogsProvisionExtension) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	// Chain with inner auth extension (e.g., sigv4auth) if configured
	if e.cfg.Auth != nil && e.host != nil {
		ext := e.host.GetExtensions()[*e.cfg.Auth]
		if ext == nil {
			return nil, fmt.Errorf("auth extension %v not found", e.cfg.Auth)
		}
		httpClient, ok := ext.(extensionauth.HTTPClient)
		if !ok {
			return nil, fmt.Errorf("auth extension %v does not implement extensionauth.HTTPClient", e.cfg.Auth)
		}
		var err error
		base, err = httpClient.RoundTripper(base)
		if err != nil {
			return nil, fmt.Errorf("failed to get RoundTripper from %v: %w", e.cfg.Auth, err)
		}
	}

	return &provisioningRoundTripper{
		base:   base,
		ext:    e,
		logger: e.logger,
	}, nil
}

// provisioningRoundTripper intercepts outgoing HTTP requests to the CW Logs OTLP
// endpoint and lazily creates the log group/stream from the request headers.
type provisioningRoundTripper struct {
	base   http.RoundTripper
	ext    *cwLogsProvisionExtension
	logger *zap.Logger
}

func (rt *provisioningRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	logGroup := req.Header.Get(headerLogGroup)
	logStream := req.Header.Get(headerLogStream)

	if logGroup != "" {
		key := logGroup + "\x00" + logStream
		if _, ok := rt.ext.provisioned.Load(key); !ok {
			// First time seeing this (group, stream) pair — provision it
			region := extractRegionFromHost(req.URL.Host)
			if region != "" {
				rt.logger.Info("Lazily provisioning CW log group/stream",
					zap.String("logGroup", logGroup),
					zap.String("logStream", logStream),
					zap.String("region", region),
				)
				if err := createLogGroupAndStream(region, logGroup, logStream); err != nil {
					rt.logger.Warn("Failed to provision CW log group/stream",
						zap.String("logGroup", logGroup),
						zap.String("logStream", logStream),
						zap.Error(err),
					)
					// Don't cache on failure — retry on next request
				} else {
					rt.ext.provisioned.Store(key, true)
				}
			}
		}
	}

	return rt.base.RoundTrip(req)
}

// extractRegionFromHost extracts the AWS region from a CW Logs endpoint host.
// e.g., "logs.us-east-1.amazonaws.com" -> "us-east-1"
func extractRegionFromHost(host string) string {
	// Expected format: logs.<region>.amazonaws.com
	if len(host) < 5 || host[:5] != "logs." {
		return ""
	}
	rest := host[5:] // "<region>.amazonaws.com"
	idx := 0
	for i, c := range rest {
		if c == '.' {
			idx = i
			break
		}
	}
	if idx == 0 {
		return ""
	}
	return rest[:idx]
}

// createLogGroupAndStream creates a CW log group and stream if they don't exist.
// Idempotent: ResourceAlreadyExistsException is silently ignored for both calls.
func createLogGroupAndStream(region, logGroupName, logStreamName string) error {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}

	svc := cloudwatchlogs.New(sess)

	// Create log group (ignore if already exists)
	_, err = svc.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("failed to create log group %q: %w", logGroupName, err)
	}

	// Create log stream (ignore if already exists)
	_, err = svc.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(logGroupName),
		LogStreamName: aws.String(logStreamName),
	})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("failed to create log stream %q in group %q: %w", logStreamName, logGroupName, err)
	}

	return nil
}

func isAlreadyExists(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == cloudwatchlogs.ErrCodeResourceAlreadyExistsException
	}
	return false
}
