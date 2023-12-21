// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

const (
	// PlatformGeneric Platforms other than Amazon EKS
	PlatformGeneric = "generic"
	// PlatformEKS Amazon EKS platform
	PlatformEKS = "eks"
)

type Resolver struct {
	Name     string `mapstructure:"name"`
	Platform string `mapstructure:"platform"`
}

func NewEKSResolver(name string) Resolver {
	return Resolver{
		Name:     name,
		Platform: PlatformEKS,
	}
}

func NewGenericResolver(name string) Resolver {
	return Resolver{
		Name:     name,
		Platform: PlatformGeneric,
	}
}
