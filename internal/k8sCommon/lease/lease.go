// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// Package lease defines shared constants for the cwagent-node-metadata Lease
// objects used by the DaemonSet LeaseWriter and the cluster-scraper
// NodeMetadataCache.
package lease

const (
	// LeasePrefix is the name prefix for node metadata Lease objects.
	LeasePrefix = "cwagent-node-metadata-"

	// Annotation keys written by the LeaseWriter and read by the NodeMetadataCache.
	AnnotationHostID   = "cwagent.amazonaws.com/host.id"
	AnnotationHostName = "cwagent.amazonaws.com/host.name"
	AnnotationHostType = "cwagent.amazonaws.com/host.type"
	AnnotationImageID  = "cwagent.amazonaws.com/host.image.id"
	AnnotationAZ       = "cwagent.amazonaws.com/cloud.availability_zone"
)
