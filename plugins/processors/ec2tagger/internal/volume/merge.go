// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package volume

import (
	"errors"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

type mergeProvider struct {
	providers []Provider
}

func newMergeProvider(providers []Provider) Provider {
	return &mergeProvider{providers: providers}
}

func (p *mergeProvider) DeviceToSerialMap() (map[string]string, error) {
	var errs error
	results := make([]map[string]string, 0, len(p.providers))
	for _, provider := range p.providers {
		if result, err := provider.DeviceToSerialMap(); err != nil {
			errs = errors.Join(errs, err)
		} else {
			results = append(results, result)
		}
	}
	if len(results) == 0 {
		return nil, errs
	}
	return collections.MergeMaps(results...), nil
}
