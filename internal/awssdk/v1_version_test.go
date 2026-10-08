// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awssdk

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

// Final aws-sdk-go v1 release. v1 reached end of support on 2025-07-31.
// Keep this floor so the agent binary cannot regress to the 1.48.x pin (#2090).
const minSDKV1 = "1.55.8"

func TestSDKGoV1Release(t *testing.T) {
	require.Equal(t, minSDKV1, aws.SDKVersion)
}
