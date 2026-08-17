// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// Package azuredetector detects Azure VM / AKS, mirroring the eksdetector package.
package azuredetector

import (
	"context"
	"sync/atomic"
	"time"

	azuremeta "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/metadataproviders/azure"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

// azureIMDSTimeout bounds one probe attempt so startup can't stall on a non-Azure host.
const azureIMDSTimeout = 1 * time.Second

// azureIMDSMaxAttempts bounds probe retries; a successful response is not retried.
const azureIMDSMaxAttempts = 2

var (
	metadataProvider = azuremeta.NewProvider()

	// IsAzureVM probes Azure IMDS (cached) and IsAKS reads RUN_IN_AKS; both are vars so they can be stubbed.
	IsAzureVM = isAzureVM
	IsAKS     = envconfig.IsRunningInAKS

	// azureVMCache holds a definitive probe result; a transient error stays uncached so boot can re-probe.
	azureVMCache atomic.Pointer[bool]
)

// isAzureVM reports whether the host is an Azure VM (cached); an unreachable probe reports false.
func isAzureVM() bool {
	ok, _ := detectAzureVM()
	return ok
}

// detectAzureVM probes Azure IMDS once, caching only a definitive result; returns the error for tests.
func detectAzureVM() (bool, error) {
	if c := azureVMCache.Load(); c != nil {
		return *c, nil
	}
	value, err := probeAzureIMDS()
	if err == nil {
		azureVMCache.CompareAndSwap(nil, &value)
	}
	return value, err
}

// probeAzureIMDS reports whether the contrib provider returns a compute doc with a VM ID; any error is retryable and uncached.
func probeAzureIMDS() (bool, error) {
	var lastErr error
	for range azureIMDSMaxAttempts {
		ctx, cancel := context.WithTimeout(context.Background(), azureIMDSTimeout)
		compute, err := metadataProvider.Metadata(ctx)
		cancel()
		if err == nil {
			// A genuine Azure compute document always carries a VM ID.
			return compute != nil && compute.VMID != "", nil
		}
		lastErr = err
	}
	return false, lastErr
}
