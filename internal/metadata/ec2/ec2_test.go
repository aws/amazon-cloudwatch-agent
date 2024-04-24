// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2

import (
	"testing"

	awsmock "github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/stretchr/testify/assert"
)

func TestNewMetadataProvider(t *testing.T) {
	mp := NewMetadataProvider(
		awsmock.Session,
		WithIMDSv2Retries(0),
	)
	cmp, ok := mp.(*chainMetadataProvider)
	assert.True(t, ok)
	assert.Len(t, cmp.providers, 3)
}
