// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// This file is NOT generated. It augments the generated partition data
// (partition.go / partitions.go) with helpers the agent needs. aws-sdk-go-v2
// does not export partition metadata, and v1's endpoints.DefaultPartitions()
// is no longer available after the SDKv2 migration.
package awsrulesfn

// PartitionDNSSuffixes returns the unique set of DNS suffixes across all AWS
// partitions (both standard and dual-stack), e.g. "amazonaws.com",
// "amazonaws.com.cn", "api.aws". Replacement for iterating
// endpoints.DefaultPartitions() and calling DNSSuffix() in SDKv1.
func PartitionDNSSuffixes() []string {
	seen := make(map[string]struct{})
	var suffixes []string
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		suffixes = append(suffixes, s)
	}
	for _, p := range partitions {
		add(p.DefaultConfig.DnsSuffix)
		add(p.DefaultConfig.DualStackDnsSuffix)
	}
	return suffixes
}
