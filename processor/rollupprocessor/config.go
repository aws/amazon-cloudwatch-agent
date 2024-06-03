// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollupprocessor

type Config struct {
	// AttributeGroups are the groups of attribute names that will be used
	// to create rollup data points with. The number of distinct groups will
	// match the number of duplicate data points that are created with those
	// attributes.
	AttributeGroups [][]string `mapstructure:"attribute_groups,omitempty"`
	// DropOriginal is the names of metrics where the original data points should
	// be dropped. This is used with the AttributeGroups to reduce the number of
	// data points sent to the exporter.
	DropOriginal []string `mapstructure:"drop_original,omitempty"`
	// CacheSize is used to store built rollup attribute groups using the base
	// attributes as keys. Can disable by setting <= 0.
	CacheSize int `mapstructure:"cache_size"`
}
