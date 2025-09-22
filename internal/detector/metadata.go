// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package detector

// Metadata represents discovered information about a resource including its categories, details, and current Status.
type Metadata struct {
	// Categories can be one or more ordered Category entries that a detector matched. For example, a Tomcat detector
	// would match both JVM and Tomcat.
	Categories []Category `json:"categories"`
	// Name is the identifier of the resource.
	Name string `json:"name,omitempty"`
	// Version is the version the detector found for the category.
	Version string `json:"version,omitempty"`
	// TelemetryPort is the port for the resource that exposes telemetry.
	TelemetryPort int `json:"telemetry_port,omitempty"`
	// Status is the current status of telemetry availability for the resource.
	Status Status `json:"status"`
}

// MetadataSlice is a grouping on Metadata entries.
type MetadataSlice []*Metadata

// Category represents a classification type for discovered resources.
type Category string

const (
	CategoryJVM    Category = "JVM"
	CategoryTomcat Category = "Tomcat"
)

// Status represents whether the resource requires more actions before telemetry is available.
type Status string

var (
	StatusReady             Status = "READY"
	StatusNeedsSetupJmxPort Status = "NEEDS_SETUP/JMX_PORT"
)
