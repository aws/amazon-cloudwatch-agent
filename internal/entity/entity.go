// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entity

// KeyPair represents a key-value pair for entity attributes
type KeyPair struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
}

// Transform contains configuration for overriding entity attributes
type Transform struct {
	KeyAttributes []KeyPair `mapstructure:"key_attributes"`
	Attributes    []KeyPair `mapstructure:"attributes"`
}
