// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// Package azuredetector detects whether the CloudWatch agent is running on
// Azure VM or Azure Kubernetes Service. Mirrors the eksdetector package.
package azuredetector

import (
	"context"
	"sync"
	"time"

	azuremeta "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/metadataproviders/azure"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

// azureIMDSTimeout bounds one probe attempt so startup can't stall on a non-Azure host.
const azureIMDSTimeout = 1 * time.Second

// azureIMDSMaxAttempts retries transient probe errors; a definitive result is not retried.
const azureIMDSMaxAttempts = 2

// IsAzureVMCache holds the cached result of the Azure VM detection probe.
type IsAzureVMCache struct {
	Value bool
	Err   error
}

var (
	// metadataProvider is the Azure IMDS provider; overridable in tests.
	metadataProvider = azuremeta.NewProvider()
	probeAzureIMDS   = defaultProbeAzureIMDS
	isRunningInAKS   = envconfig.IsRunningInAKS

	// Public detection entry points; overridable in tests (mirrors eksdetector.IsEKS).
	IsAzureVM = isAzureVM
	IsAKS     = isAKS

	// Azure VM detection cache; mutex (not sync.Once) so tests can reset it safely.
	azureVMMu       sync.Mutex
	azureVMResolved bool
	azureVMCache    IsAzureVMCache
)

// isAKS reports whether RUN_IN_AKS is set (by the AKS Helm chart); no I/O.
func isAKS() bool {
	return isRunningInAKS()
}

// isAzureVM probes Azure IMDS (cached). An unreachable probe reports false.
func isAzureVM() IsAzureVMCache {
	azureVMMu.Lock()
	defer azureVMMu.Unlock()
	if !azureVMResolved {
		value, err := probeAzureIMDS()
		result := IsAzureVMCache{Value: value, Err: err}
		// Cache only a definitive answer so a boot-time transient error can re-probe.
		if err == nil {
			azureVMCache = result
			azureVMResolved = true
		}
		return result
	}
	return azureVMCache
}

// defaultProbeAzureIMDS queries the Azure IMDS via the contrib metadata provider,
// returning true when a compute document with a VM ID is returned. The provider
// error is transient (IMDS unreachable), so it is retried but never cached.
func defaultProbeAzureIMDS() (bool, error) {
	var lastErr error
	for attempt := 0; attempt < azureIMDSMaxAttempts; attempt++ {
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
