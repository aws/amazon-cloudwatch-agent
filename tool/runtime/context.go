// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package runtime

type Context struct {
	OsParameter               string //we will do all the validation when setting this OsParameter value, skip all the validation afterwards.
	IsOnPrem                  bool
	WantPerInstanceMetrics    bool //CPU per core
	WantEC2TagDimensions      bool
	WantAggregateDimensions   bool
	MetricsCollectionInterval int //sub minute, high resolution, metric collect interval, unit as sec.
	ConfigOutputPath          string

	//linux migration
	HasExistingLinuxConfig bool
	ConfigFilePath         string

	//windows migration
	WindowsNonInteractiveMigration bool

	//Xray Daemon Migration
	TracesOnly                  bool
	NonInteractiveXrayMigration bool
}
