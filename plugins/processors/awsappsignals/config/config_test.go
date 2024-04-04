// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePassed(t *testing.T) {
	config := Config{
		Resolvers: []Resolver{NewEKSResolver("test"), NewGenericResolver("")},
		Rules:     nil,
	}
	assert.Nil(t, config.Validate())

	config = Config{
		Resolvers: []Resolver{NewK8sResolver("test"), NewGenericResolver("")},
		Rules:     nil,
	}
	assert.Nil(t, config.Validate())

	config = Config{
		Resolvers: []Resolver{NewEC2Resolver("test"), NewGenericResolver("")},
		Rules:     nil,
	}
	assert.Nil(t, config.Validate())
}

func TestValidateFailedOnEmptyResolver(t *testing.T) {
	config := Config{
		Resolvers: []Resolver{},
		Rules:     nil,
	}
	assert.NotNil(t, config.Validate())
}

func TestValidateFailedOnEmptyResolverName(t *testing.T) {
	config := Config{
		Resolvers: []Resolver{NewEKSResolver("")},
		Rules:     nil,
	}
	assert.NotNil(t, config.Validate())

	config = Config{
		Resolvers: []Resolver{NewK8sResolver("")},
		Rules:     nil,
	}
	assert.NotNil(t, config.Validate())
}
