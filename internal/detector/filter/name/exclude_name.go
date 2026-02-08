// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package name

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

type excludeFilter struct {
	lookup collections.Set[string]
}

func NewExcludeFilter(names ...string) detector.NameFilter {
	return &excludeFilter{lookup: collections.NewSet[string](names...)}
}

func (f *excludeFilter) ShouldInclude(name string) bool {
	return !f.lookup.Contains(name)
}
