// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package constants

const (
	SectionKeyLogsCollected = "logs_collected"
)

// children of logs_collected
const (
	SectionKeyFiles = "files"
)

// child of files
const (
	SectionKeyCollectList = "collect_list"
)

// children of collect_list
const (
	SectionKeyFilePath        = "file_path"
	SectionKeyTimestampFormat = "timestamp_format"
)
