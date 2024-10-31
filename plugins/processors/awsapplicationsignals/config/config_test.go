// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePassed(t *testing.T) {
	tests := []struct {
		name     string
		resolver Resolver
	}{
		{
			"testEKS",
			NewEKSResolver("test"),
		},
		{
			"testK8S",
			NewK8sResolver("test"),
		},
		{
			"testEC2",
			NewEC2Resolver("test"),
		},
		{
			"testECS",
			NewECSResolver("test"),
		},
		{
			"testGeneric",
			NewGenericResolver("test"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Resolvers: []Resolver{tt.resolver},
				Rules:     nil,
			}
			assert.Nil(t, config.Validate())

		})
	}
}

func TestValidateFailedOnEmptyResolver(t *testing.T) {
	config := Config{
		Resolvers: []Resolver{},
		Rules:     nil,
	}
	assert.NotNil(t, config.Validate())
}

func TestValidateFailedOnEmptyResolverName(t *testing.T) {
	tests := []struct {
		name     string
		resolver Resolver
	}{
		{
			"testEKS",
			NewEKSResolver(""),
		},
		{
			"testK8S",
			NewK8sResolver(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Resolvers: []Resolver{tt.resolver},
				Rules:     nil,
			}
			assert.NotNil(t, config.Validate())

		})
	}
}
