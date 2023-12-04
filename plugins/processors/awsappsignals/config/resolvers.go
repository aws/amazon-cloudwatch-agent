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
	ClusterName string `mapstructure:"cluster_name"`
	Platform    string `mapstructure:"platform"`
}

func NewEKSResolver(clusterName string) Resolver {
	return Resolver{
		ClusterName: clusterName,
		Platform:    PlatformEKS,
	}
}

func NewGenericResolver() Resolver {
	return Resolver{
		Platform: PlatformGeneric,
	}
}
