// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

var (
	// TestIsEKSCacheEKS is used for unit testing EKS route
	TestIsEKSCacheEKS = func() IsEKSCache {
		return IsEKSCache{Value: true, Err: nil}
	}

	// TestIsEKSCacheK8s is used for unit testing K8s route
	TestIsEKSCacheK8s = func() IsEKSCache {
		return IsEKSCache{Value: false, Err: nil}
	}
)
