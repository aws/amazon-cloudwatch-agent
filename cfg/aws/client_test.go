// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

func TestGetSharedHTTPClient_Concurrent(t *testing.T) {
	sharedHTTPClientOnce = sync.Once{}
	sharedHTTPClient = nil

	const count = 100
	clients := make([]aws.HTTPClient, count)
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			clients[index] = getSharedHTTPClient()
		}(i)
	}

	wg.Wait()

	// All goroutines should get the same instance
	first := clients[0]
	assert.NotNil(t, first)
	for i := 1; i < count; i++ {
		assert.Same(t, first, clients[i])
	}
}
