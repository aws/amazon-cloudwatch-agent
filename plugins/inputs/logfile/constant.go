// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"time"
)

const sampleConfig = `
  ## log files to tail.
  ## These accept standard unix glob matching rules, but with the addition of
  ## ** as a "super asterisk". ie:
  ##   "/var/log/**.log"  -> recursively find all .log files in /var/log
  ##   "/var/log/*/*.log" -> find all .log files with a parent dir in /var/log
  ##   "/var/log/apache.log" -> just tail the apache log file
  ##
  ## See https://github.com/gobwas/glob for more examples
  ##
  ## Default log output destination name for all file_configs
  ## each file_config can override its own destination if needed
  destination = "cloudwatchlogs"

  ## folder path where state of how much of a file has been transferred is stored
  file_state_folder = "/tmp/logfile/state"

  [[inputs.logs.file_config]]
      file_path = "/tmp/logfile.log*"
      ## Regular expression for log files to ignore
      blacklist = "logfile.log.bak"
      ## Publish all log files that match file_path
      publish_multi_logs = false
      log_group_name = "logfile.log"
      log_stream_name = "<log_stream_name>"
      publish_multi_logs = false
      timestamp_regex = "^(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}).*$"
      timestamp_layout = "02 Jan 2006 15:04:05"
      timezone = "UTC"
      multi_line_start_pattern = "{timestamp_regex}"
      ## Read file from beginning.
      from_beginning = false
      ## Whether file is a named pipe
      pipe = false
      destination = "cloudwatchlogs"
      ## Max size of each log event, defaults to 262144 (256KB)
      max_event_size = 262144
      ## Suffix to be added to truncated logline to indicate its truncation, defaults to "[Truncated...]"
      truncate_suffix = "[Truncated...]"
`

const defaultTimeoutToAcquire = 100 * time.Millisecond