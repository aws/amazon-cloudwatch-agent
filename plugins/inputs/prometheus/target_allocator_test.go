// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/promslog"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
)

// newUnreachableTAManager builds a TargetAllocatorManager against the testdata
// config, whose target allocator endpoint (http://target-allocator-service:80)
// does not resolve on a test host, so the manager's initial sync retries with
// backoff until its context is cancelled.
func newUnreachableTAManager(t *testing.T) (*TargetAllocatorManager, func()) {
	t.Helper()
	t.Setenv("POD_NAME", "collector-1")

	logLevel := promslog.NewLevel()
	require.NoError(t, logLevel.Set("info"))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	registry := prometheus.NewRegistry()
	ctxScrape, cancelScrape := context.WithCancel(context.Background())
	sdMetrics, err := discovery.CreateAndRegisterSDMetrics(registry)
	require.NoError(t, err)
	dm := discovery.NewManager(ctxScrape, logger, registry, sdMetrics, discovery.Name("scrape"))
	sm, err := scrape.NewManager(&scrape.Options{}, logger, nil, &metricsReceiver{}, nil, registry)
	require.NoError(t, err)

	tam := createTargetAllocatorManager(filepath.Join("testdata", "target_allocator.yaml"), logger, logLevel, sm, dm)
	require.True(t, tam.enabled)
	// The production TLS file paths do not exist on a test host; clear them so
	// Start reaches the initial-sync retry loop instead of failing client setup.
	tam.config.TargetAllocator.Get().TLS = configtls.ClientConfig{}
	tam.loadManager(logLevel)
	tam.AttachReloadConfigHandler(func(_ *promconfig.Config) {})
	return tam, cancelScrape
}

// TestTargetAllocatorRunStartBounded verifies Run does not stall indefinitely on
// an unreachable target allocator: the upstream manager's initial sync retries
// with backoff for up to ~15 minutes unless its context is cancelled.
func TestTargetAllocatorRunStartBounded(t *testing.T) {
	oldTimeout := taStartTimeout
	taStartTimeout = 2 * time.Second
	defer func() { taStartTimeout = oldTimeout }()

	tam, cancelScrape := newUnreachableTAManager(t)
	defer cancelScrape()

	runDone := make(chan error, 1)
	go func() { runDone <- tam.Run() }()

	// Run blocks on shutdownCh after a successful (soft-failed) Start; the key
	// assertion is that dependents are unblocked (taReadyCh closes) well before
	// the upstream ~15m retry window.
	select {
	case <-tam.taReadyCh:
		// Start was bounded by taStartTimeout and Run proceeded.
	case err := <-runDone:
		t.Fatalf("Run returned before ready: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("Run still blocked in Start after 30s; startup stall not bounded")
	}
	tam.Shutdown()
	select {
	case <-runDone:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after Shutdown")
	}
}

// TestTargetAllocatorShutdownInterruptsStart verifies Shutdown releases a Run
// that is still blocked in the initial sync against an unreachable allocator.
func TestTargetAllocatorShutdownInterruptsStart(t *testing.T) {
	tam, cancelScrape := newUnreachableTAManager(t)
	defer cancelScrape()

	runDone := make(chan error, 1)
	go func() { runDone <- tam.Run() }()

	// Give Start a moment to enter the retry loop, then shut down.
	time.Sleep(2 * time.Second)
	tam.Shutdown()

	select {
	case <-runDone:
		// Run returned promptly; error or nil are both acceptable here — the
		// guarantee under test is liveness, not the sync outcome.
	case <-time.After(10 * time.Second):
		t.Fatal("Run still blocked 10s after Shutdown; initial sync not interruptible")
	}
}
