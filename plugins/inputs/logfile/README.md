# Logs Input Plugin

The logs plugin "tails" a logfile and parses each log message.

By default, the tail plugin acts like the following unix tail command:

```
tail -F --lines=0 myfile.log
```

- `-F` means that it will follow the _name_ of the given file, so
that it will be compatible with log-rotated files, and that it will retry on
inaccessible files.
- `--lines=0` means that it will start at the end of the file (unless
the `from_beginning` option is set).

see http://man7.org/linux/man-pages/man1/tail.1.html for more details.

The plugin expects messages in one of the
[Telegraf Input Data Formats](https://github.com/influxdata/telegraf/blob/master/docs/DATA_FORMATS_INPUT.md).

### Configuration:

```toml
# Stream a log file, like the tail -f command
  [[inputs.logs]]
  ## files to tail.
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
      log_group_name = "logfile.log"
      log_stream_name = "<log_stream_name>"
      timestamp_regex = "^(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}).*$"
      timestamp_layout = ["_2 Jan 2006 15:04:05"]
      timezone = "UTC"
      multi_line_start_pattern = "{timestamp_regex}"
      ## Read file from beginning.
      from_beginning = false
      ## Whether file is a named pipe
      pipe = false
      retention_in_days = -1
      destination = "cloudwatchlogs"
  [[inputs.logs.file_config]]
      file_path = "/var/log/*.log"
      ## Regular expression for log files to ignore
      blacklist = "journal|syslog"
      ## Publish all log files that match file_path
      publish_multi_logs = true
      log_group_name = "varlog"
      log_stream_name = "<log_stream_name>"
      timestamp_regex = "^(\\d{2} \\w{3} \\d{4} \\d{2}:\\d{2}:\\d{2}).*$"
      timestamp_layout = ["_2 Jan 2006 15:04:05"]
      timezone = "UTC"
      multi_line_start_pattern = "{timestamp_regex}"
      ## Read file from beginning.
      from_beginning = false
      ## Whether file is a named pipe
      pipe = false
      retention_in_days = -1
      destination = "cloudwatchlogs"
      ## Max size of each log event, defaults to 262144 (256KB)
      max_event_size = 262144
      ## Suffix to be added to truncated logline to indicate its truncation, defaults to "[Truncated...]"
      truncate_suffix = "[Truncated...]"

```

