// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/pdata/pcommon"

	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/internal/attributes"
)

type MockSubResolver struct {
	mock.Mock
}

func (m *MockSubResolver) Process(attributes, resourceAttributes pcommon.Map) error {
	args := m.Called(attributes, resourceAttributes)
	return args.Error(0)
}

func (m *MockSubResolver) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestHostedInAttributeResolverWithNoConfiguredName(t *testing.T) {
	resolver := newHostedInAttributeResolver("", DefaultHostedInAttributes)

	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEnvironment)
	assert.True(t, ok)
	assert.Equal(t, "Generic", envAttr.AsString())
}

func TestHostedInAttributeResolverWithConfiguredName(t *testing.T) {
	resolver := newHostedInAttributeResolver("test", DefaultHostedInAttributes)

	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEnvironment)
	assert.True(t, ok)
	assert.Equal(t, "test", envAttr.AsString())
}

func TestHostedInAttributeResolverWithConflictedName(t *testing.T) {
	resolver := newHostedInAttributeResolver("test", DefaultHostedInAttributes)

	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()
	resourceAttributes.PutStr(attr.AWSHostedInEnvironment, "self-defined")

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.HostedInEnvironment)
	assert.True(t, ok)
	assert.Equal(t, "self-defined", envAttr.AsString())
}

func TestHostedInAttributeResolverWithHostname(t *testing.T) {
	resolver := newHostedInAttributeResolver("test", DefaultHostedInAttributes)

	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()
	resourceAttributes.PutStr(attr.ResourceDetectionHostName, "hostname")

	resolver.Process(attributes, resourceAttributes)
	envAttr, ok := attributes.Get(attr.ResourceDetectionHostName)
	assert.True(t, ok)
	assert.Equal(t, "hostname", envAttr.AsString())
}

func TestAttributesResolver_Process(t *testing.T) {
	attributes := pcommon.NewMap()
	resourceAttributes := pcommon.NewMap()

	mockSubResolver1 := new(MockSubResolver)
	mockSubResolver1.On("Process", attributes, resourceAttributes).Return(nil)

	mockSubResolver2 := new(MockSubResolver)
	mockSubResolver2.On("Process", attributes, resourceAttributes).Return(errors.New("error"))

	r := &attributesResolver{
		subResolvers: []subResolver{mockSubResolver1, mockSubResolver2},
	}

	err := r.Process(attributes, resourceAttributes, true)
	assert.Error(t, err)
	mockSubResolver1.AssertExpectations(t)
	mockSubResolver2.AssertExpectations(t)
}

func TestAttributesResolver_Stop(t *testing.T) {
	ctx := context.Background()

	mockSubResolver1 := new(MockSubResolver)
	mockSubResolver1.On("Stop", ctx).Return(nil)

	mockSubResolver2 := new(MockSubResolver)
	mockSubResolver2.On("Stop", ctx).Return(errors.New("error"))

	r := &attributesResolver{
		subResolvers: []subResolver{mockSubResolver1, mockSubResolver2},
	}

	err := r.Stop(ctx)
	assert.Error(t, err)
	mockSubResolver1.AssertExpectations(t)
	mockSubResolver2.AssertExpectations(t)
}
