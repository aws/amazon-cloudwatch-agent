// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata/azure"
)

var (
	globalProvider Provider
	once           sync.Once
)

const detectTimeout = 3 * time.Second

// GetProvider returns the singleton cloud metadata provider.
// It auto-detects the cloud by trying each provider in order.
// Detection uses a short timeout to avoid blocking agent startup
// on hosts where IMDS is unreachable.
func GetProvider() Provider {
	once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), detectTimeout)
		defer cancel()

		// Try AWS first (most common)
		if p, err := aws.NewProvider(ctx); err == nil {
			globalProvider = p
			log.Printf("I! [cloudmetadata] Detected AWS (region=%s, instanceID=%s)\n", globalProvider.Region(), globalProvider.InstanceID())
			return
		}

		// Try Azure
		if p, err := azure.NewProvider(ctx); err == nil {
			globalProvider = p
			log.Printf("I! [cloudmetadata] Detected Azure (region=%s, instanceID=%s)\n", globalProvider.Region(), globalProvider.InstanceID())
			return
		}

		// No cloud detected
		log.Println("I! [cloudmetadata] No cloud provider detected")
	})
	return globalProvider
}

// ResetForTest resets the singleton for testing.
func ResetForTest() {
	once = sync.Once{}
	globalProvider = nil
}

// SetForTest sets a specific provider for testing.
// Pass nil to simulate no cloud detected.
func SetForTest(p Provider) {
	once = sync.Once{}
	once.Do(func() {})
	globalProvider = p
}
