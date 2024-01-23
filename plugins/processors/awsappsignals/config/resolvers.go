// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

const (
	// PlatformGeneric Platforms other than Amazon EKS
	PlatformGeneric = "generic"
	// PlatformEKS Amazon EKS platform
	PlatformEKS = "eks"
	// PlatformK8s Native Kubernetes
	PlatformK8s = "k8s"
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

func NewK8sResolver(name string) Resolver {
	return Resolver{
		Name:     name,
		Platform: PlatformK8s,
	}
}

func NewGenericResolver(name string) Resolver {
	return Resolver{
		Name:     name,
		Platform: PlatformGeneric,
	}
}
