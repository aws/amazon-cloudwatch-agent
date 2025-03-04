// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logscommon

const (
	LogGroupNameTag  = "log_group_name"
	LogStreamNameTag = "log_stream_name"
	VersionTag       = "Version"
	TimestampTag     = "Timestamp"

	//Field key in metrics indicting if the line is the start of the multiline.
	//If this key is not present, it means the multiline mode is not enabled,
	//    we set it to true, it indicates it is a real event, but not part of a multiple line.
	//If this key is false, it means the line is not start line of multiline entry.
	//If this key is true, it means the line is the start of multiline entry.
	MultiLineStartField = "multi_line_start"
	//Field key in metrics passing the offset for the current input file.
	OffsetField = "offset"
	//Field key in metrics passing the file name.
	FileNameField = "file_name"
	//This field to store the timestampFromLogLine parsed from the log entry if the log entry has timestampFromLogLine associated.
	LogTimestampField = "log_timestamp"
	//The field key in the metrics for the log entry content
	LogEntryField = "value"

	WindowsEventLogPrefix = "Amazon_CloudWatch_WindowsEventLog_"
	LogType               = "log_type"
)
