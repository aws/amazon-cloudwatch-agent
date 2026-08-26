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

// Azure platform modes: ModeAzureVM is host-level (like ModeEC2), ModeAKS is Kubernetes-level (like ModeEKS).
const (
	ModeAzureVM = "AzureVM"
	ModeAKS     = "AKS"
)

// ModeGCE is the GCP host-level mode (like ModeEC2).
const (
	ModeGCE = "GCE"
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
	ShortModeGCE       = "GCE"
)

// ModeDefersRegion reports whether the mode has no AWS region source at translation time and instead resolves
// the region from the AWS_REGION environment variable at runtime. This is true for non-AWS cloud hosts (Azure
// VM, GCE), which have no AWS IMDS to detect a region from.
func ModeDefersRegion(mode string) bool {
	return mode == ModeAzureVM || mode == ModeGCE
}
