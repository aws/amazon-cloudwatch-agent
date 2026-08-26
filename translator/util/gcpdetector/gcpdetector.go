// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gcpdetector

import (
	"cloud.google.com/go/compute/metadata"
)

// IsGCE probes the GCE metadata server (cached by the SDK). It is a var so it can be stubbed in tests.
var IsGCE = metadata.OnGCE
