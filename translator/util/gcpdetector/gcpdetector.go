// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gcpdetector

import (
	"cloud.google.com/go/compute/metadata"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

// IsGCE probes the GCE metadata server (cached by the SDK). It is a var so it can be stubbed in tests.
var IsGCE = metadata.OnGCE

// IsGKE reads RUN_IN_GKE (set by the GKE Helm chart) so GKE is detected without a metadata probe. It is a
// var so it can be stubbed in tests.
var IsGKE = envconfig.IsRunningInGKE
