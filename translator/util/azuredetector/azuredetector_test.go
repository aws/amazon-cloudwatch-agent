// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azuredetector

import (
	"context"
	"errors"
	"testing"

	azuremeta "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/metadataproviders/azure"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

// fakeProvider is a stub azuremeta.Provider for tests.
type fakeProvider struct {
	compute *azuremeta.ComputeMetadata
	err     error
	calls   int
}

func (f *fakeProvider) Metadata(context.Context) (*azuremeta.ComputeMetadata, error) {
	f.calls++
	return f.compute, f.err
}

// setProvider swaps metadataProvider for the test and resets the cache,
// restoring both on cleanup.
func setProvider(t *testing.T, p azuremeta.Provider) {
	t.Helper()
	orig := metadataProvider
	metadataProvider = p
	resetCacheForTesting()
	t.Cleanup(func() {
		metadataProvider = orig
		resetCacheForTesting()
	})
}

// resetCacheForTesting clears the Azure VM detection cache under its lock.
func resetCacheForTesting() {
	azureVMMu.Lock()
	defer azureVMMu.Unlock()
	azureVMCache = IsAzureVMCache{}
	azureVMResolved = false
}

// ---- AKS (env-var) detection ----

func TestIsAKS_EnvSet(t *testing.T) {
	orig := isRunningInAKS
	t.Cleanup(func() { isRunningInAKS = orig })
	isRunningInAKS = func() bool { return true }
	assert.True(t, isAKS())
}

func TestIsAKS_EnvUnset(t *testing.T) {
	orig := isRunningInAKS
	t.Cleanup(func() { isRunningInAKS = orig })
	isRunningInAKS = func() bool { return false }
	assert.False(t, isAKS())
}

func TestIsAKS_ReadsRunInAKSEnvVar(t *testing.T) {
	// Exercise the real envconfig-backed check to confirm the wiring to RUN_IN_AKS.
	t.Setenv(envconfig.RunInAKS, envconfig.TrueValue)
	assert.True(t, isAKS())

	t.Setenv(envconfig.RunInAKS, "")
	assert.False(t, isAKS())
}

// ---- Azure VM (IMDS) detection ----

func TestIsAzureVM_Detected(t *testing.T) {
	setProvider(t, &fakeProvider{compute: &azuremeta.ComputeMetadata{VMID: "vm-123", Name: "vm1"}})

	result := isAzureVM()
	assert.True(t, result.Value)
	assert.NoError(t, result.Err)
}

func TestIsAzureVM_NotAzure(t *testing.T) {
	// The provider succeeds but returns no VM ID (e.g. a non-Azure 200 response).
	cases := map[string]*azuremeta.ComputeMetadata{
		"nilCompute":   nil,
		"emptyCompute": {},
	}
	for name, compute := range cases {
		t.Run(name, func(t *testing.T) {
			setProvider(t, &fakeProvider{compute: compute})
			result := isAzureVM()
			assert.False(t, result.Value)
			assert.NoError(t, result.Err)
		})
	}
}

func TestIsAzureVM_Unreachable(t *testing.T) {
	// IMDS unreachable (e.g. on-prem/EC2): the probe errors and reports false.
	p := &fakeProvider{err: errors.New("dial tcp 169.254.169.254: connect: no route to host")}
	setProvider(t, p)

	result := isAzureVM()
	assert.False(t, result.Value)
	assert.Error(t, result.Err)
	assert.Equal(t, azureIMDSMaxAttempts, p.calls, "a transient error should be retried")
}

func TestIsAzureVM_TransientErrorNotCached(t *testing.T) {
	// A probe that exhausts retries with a transient error must NOT cache a
	// negative: a later call (IMDS now up) must be able to re-probe.
	p := &transientThenOKProvider{failFirst: azureIMDSMaxAttempts}
	setProvider(t, p)

	first := isAzureVM()
	assert.False(t, first.Value)
	assert.Error(t, first.Err)
	assert.Equal(t, azureIMDSMaxAttempts, p.calls, "first call should exhaust all retry attempts")

	second := isAzureVM()
	assert.True(t, second.Value, "a transient failure must not be cached; re-probe should succeed")
	assert.NoError(t, second.Err)
	assert.Equal(t, azureIMDSMaxAttempts+1, p.calls, "second call should require exactly one successful probe")
}

func TestIsAzureVM_ResultIsCached(t *testing.T) {
	p := &fakeProvider{compute: &azuremeta.ComputeMetadata{VMID: "vm-123"}}
	setProvider(t, p)

	first := isAzureVM()
	second := isAzureVM()
	assert.True(t, first.Value)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, p.calls, "IMDS should be queried at most once due to caching")
}

// transientThenOKProvider errors for the first failFirst calls, then succeeds.
type transientThenOKProvider struct {
	failFirst int
	calls     int
}

func (p *transientThenOKProvider) Metadata(context.Context) (*azuremeta.ComputeMetadata, error) {
	p.calls++
	if p.calls <= p.failFirst {
		return nil, errors.New("imds temporarily unavailable")
	}
	return &azuremeta.ComputeMetadata{VMID: "vm-123"}, nil
}
