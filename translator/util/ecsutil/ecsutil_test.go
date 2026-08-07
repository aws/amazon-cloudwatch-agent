// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRegion_ValidARN_ShortFormat(t *testing.T) {
	e := &ecsUtil{}
	em := &ecsMetadataResponse{TaskARN: "arn:aws:ecs:us-east-1:123456789012:task/abc123"}
	e.parseRegion(em)
	assert.Equal(t, "us-east-1", e.Region)
}

func TestParseRegion_ValidARN_LongFormat(t *testing.T) {
	// Long-form ARN includes the cluster name segment before the task id.
	e := &ecsUtil{}
	em := &ecsMetadataResponse{TaskARN: "arn:aws:ecs:eu-west-2:123456789012:task/my-cluster/abc123"}
	e.parseRegion(em)
	assert.Equal(t, "eu-west-2", e.Region)
}

func TestParseRegion_InvalidARN_TooFewSegments(t *testing.T) {
	e := &ecsUtil{}
	em := &ecsMetadataResponse{TaskARN: "not-an-arn"}
	require.NotPanics(t, func() { e.parseRegion(em) })
	assert.Equal(t, "", e.Region)
}

func TestParseRegion_InvalidARN_Empty(t *testing.T) {
	e := &ecsUtil{}
	em := &ecsMetadataResponse{TaskARN: ""}
	require.NotPanics(t, func() { e.parseRegion(em) })
	assert.Equal(t, "", e.Region)
}
