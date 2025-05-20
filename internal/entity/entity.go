// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entity

// KeyPair represents a key-value pair for entity attributes
type KeyPair struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
}

// EntityTransform contains configuration for overriding entity attributes
type EntityTransform struct {
	KeyAttributes []KeyPair `mapstructure:"key_attributes"`
	Attributes    []KeyPair `mapstructure:"attributes"`
}
