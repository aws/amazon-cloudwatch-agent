// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

const (
	ModeEC2       = "ec2"
	ModeOnPrem    = "onPrem"
	ModeOnPremise = "onPremise"
	ModeWithIRSA  = "withIRSA"
)

const (
	ModeECS = "ECS"
)

const (
	ModeEKS       = "EKS"
	ModeK8sEC2    = "K8sEC2"
	ModeK8sOnPrem = "K8sOnPrem"
)

// Azure platform modes. ModeAzureVM is a host-level mode analogous to
// ModeEC2; ModeAKS is a Kubernetes-level mode analogous to ModeEKS.
const (
	ModeAzureVM = "AzureVM"
	ModeAKS     = "AKS"
)

const (
	ShortModeEC2       = "EC2"
	ShortModeOnPrem    = "OP"
	ShortModeWithIRSA  = "WI"
	ShortModeEKS       = "EKS"
	ShortModeK8sEC2    = "K8E"
	ShortModeK8sOnPrem = "K8OP"
	ShortModeAzureVM   = "AZVM"
	ShortModeAKS       = "AKS"
)
