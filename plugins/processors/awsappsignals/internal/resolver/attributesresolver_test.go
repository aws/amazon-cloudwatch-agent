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
