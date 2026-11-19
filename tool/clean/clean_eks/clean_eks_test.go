// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"testing"
	"time"
)

func TestClusterNameMatchesClustersToClean(t *testing.T) {
	prefixes := []string{"cwagent-eks-integ-", "cwagent-addon-eks-integ-"}
	tests := []struct {
		name        string
		clusterName string
		want        bool
	}{
		{"matches first prefix", "cwagent-eks-integ-abc123", true},
		{"matches second prefix", "cwagent-addon-eks-integ-xyz", true},
		{"no match", "some-other-cluster", false},
		{"empty name", "", false},
		{"prefix only as substring, not prefix", "x-cwagent-eks-integ-abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clusterNameMatchesClustersToClean(tt.clusterName, prefixes); got != tt.want {
				t.Errorf("clusterNameMatchesClustersToClean(%q) = %v, want %v", tt.clusterName, got, tt.want)
			}
		})
	}
}

func TestShouldCleanCluster(t *testing.T) {
	prefixes := []string{"cwagent-eks-integ-"}
	now := time.Now().UTC()
	expiration := now.Add(-24 * time.Hour) // anything created before 24h ago is cleanable

	old := now.Add(-48 * time.Hour)
	recent := now.Add(-1 * time.Hour)

	tests := []struct {
		name        string
		clusterName string
		createdAt   *time.Time
		want        bool
	}{
		{"old and matching prefix is cleanable", "cwagent-eks-integ-abc", &old, true},
		{"recent and matching prefix is kept", "cwagent-eks-integ-abc", &recent, false},
		{"old but non-matching prefix is kept", "unrelated-cluster", &old, false},
		{"nil createdAt is kept", "cwagent-eks-integ-abc", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldCleanCluster(tt.clusterName, tt.createdAt, expiration, prefixes); got != tt.want {
				t.Errorf("shouldCleanCluster(%q) = %v, want %v", tt.clusterName, got, tt.want)
			}
		})
	}
}
