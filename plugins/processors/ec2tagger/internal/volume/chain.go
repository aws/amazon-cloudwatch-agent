// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package volume

import (
	"errors"
	"log"
)

type chainProvider struct {
	providers []Provider
}

func newChainProvider(providers []Provider) Provider {
	return &chainProvider{providers: providers}
}

func (p *chainProvider) DeviceToSerialMap() (map[string]string, error) {
	var errs error
	for _, provider := range p.providers {
		if result, err := provider.DeviceToSerialMap(); err != nil {
			log.Printf("%T: %v", provider, err)
			errs = errors.Join(errs, err)
		} else {
			return result, nil
		}
	}
	return nil, errs
}
