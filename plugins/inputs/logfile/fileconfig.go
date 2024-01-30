// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/profiler"
)

const (
	defaultMaxEventSize   = 1024 * 256 //256KB
	defaultTruncateSuffix = "[Truncated...]"
)

// The file config presents the structure of configuration for a file to be tailed.
type FileConfig struct {
	//The file path for input log file.
	FilePath string `toml:"file_path"`
	//The blacklist used to filter out some files
	Blacklist string `toml:"blacklist"`

	PublishMultiLogs bool `toml:"publish_multi_logs"`

	Encoding string `toml:"encoding"`
	//The log group name for the input log file.
	LogGroupName string `toml:"log_group_name"`
	//log stream name
	LogStreamName string `toml:"log_stream_name"`
	//log group class
	LogGroupClass string `toml:"log_group_class"`

	//The regex of the timestampFromLogLine presents in the log entry
	TimestampRegex string `toml:"timestamp_regex"`
	//The timestampFromLogLine layout used in GoLang to parse the timestampFromLogLine.
	TimestampLayout []string `toml:"timestamp_layout"`
	//The time zone used to parse the timestampFromLogLine in the log entry.
	Timezone string `toml:"timezone"`

	//Indicate whether it is a start of multiline.
	//If this config is not present, it means the multiline mode is disabled.
	//If this config is specified as "{timestamp_regex}", it means to use the same regex as timestampFromLogLine.
	//If this config is specified as some regex, it will use the regex to determine if this line is a start line of multiline entry.
	MultiLineStartPattern string `toml:"multi_line_start_pattern"`

	// automatically remove the file / symlink after uploading.
	// This auto removal does not support the case where other log rotation mechanism is already in place.
	AutoRemoval bool `toml:"auto_removal"`

	//Indicate whether to tail the log file from the beginning or not.
	//The default value for this field should be set as true in configuration.
	//Otherwise, it may skip some log entries for timestampFromLogLine suffix roatated new file.
	FromBeginning bool `toml:"from_beginning"`
	//Indicate whether it is a named pipe.
	Pipe bool `toml:"pipe"`

	//Indicate logType for scroll
	LogType string `toml:"log_type"`

	//Log Destination override
	Destination string `toml:"destination"`

	//Max size for a single log event to be in bytes
	MaxEventSize int `toml:"max_event_size"`

	//Suffix to be added to truncated logline to indicate its truncation
	TruncateSuffix string `toml:"truncate_suffix"`

	//Indicate retention in days for log group
	RetentionInDays int `toml:"retention_in_days"`

	Filters []*LogFilter `toml:"filters"`

	//Time *time.Location Go type timezone info.
	TimezoneLoc *time.Location
	//Regexp go type timestampFromLogLine regex
	TimestampRegexP *regexp.Regexp
	//Regexp go type multiline start regex
	MultiLineStartPatternP *regexp.Regexp
	//Regexp go type blacklist regex
	BlacklistRegexP *regexp.Regexp
	//Decoder object
	Enc         encoding.Encoding
	sampleCount int
}

// Initialize some variables in the FileConfig object based on the rest info fetched from the configuration file.
func (config *FileConfig) init() error {
	var err error
	if !(config.Encoding == "" || config.Encoding == "utf_8" || config.Encoding == "utf-8" || config.Encoding == "utf8" || config.Encoding == "ascii") {
		if config.Enc, _ = charset.Lookup(config.Encoding); config.Enc == nil {
			if config.Enc, _ = ianaindex.IANA.Encoding(config.Encoding); config.Enc == nil {
				msg := fmt.Sprintf("E! the encoding %s is not supported.", config.Encoding)
				log.Printf(msg)
				return errors.New(msg)
			}
		}
	}
	//If the log group name is not specified, we will use the part before the last dot in the file path as the log group name.
	if config.LogGroupName == "" && !config.PublishMultiLogs {
		config.LogGroupName = logGroupName(config.FilePath)
	}
	//If the timezone info is not specified, we will use the Local timezone as default value.
	if config.Timezone == time.UTC.String() {
		config.TimezoneLoc = time.UTC
	} else {
		config.TimezoneLoc = time.Local
	}

	if config.TimestampRegex != "" {
		if config.TimestampRegexP, err = regexp.Compile(config.TimestampRegex); err != nil {
			return fmt.Errorf("timestamp_regex has issue, regexp: Compile( %v ): %v", config.TimestampRegex, err.Error())
		}
	}

	if config.MultiLineStartPattern == "" {
		config.MultiLineStartPattern = "^[\\S]"
	}
	if config.MultiLineStartPattern == "{timestamp_regex}" {
		config.MultiLineStartPatternP = config.TimestampRegexP
	} else {
		if config.MultiLineStartPatternP, err = regexp.Compile(config.MultiLineStartPattern); err != nil {
			return fmt.Errorf("multi_line_start_pattern has issue, regexp: Compile( %v ): %v", config.MultiLineStartPattern, err.Error())
		}
	}

	if config.Blacklist != "" {
		if config.BlacklistRegexP, err = regexp.Compile(config.Blacklist); err != nil {
			return fmt.Errorf("blacklist regex has issue, regexp: Compile( %v ): %v", config.Blacklist, err.Error())
		}
	}

	if config.MaxEventSize == 0 {
		config.MaxEventSize = defaultMaxEventSize
	}

	if config.TruncateSuffix == "" {
		config.TruncateSuffix = defaultTruncateSuffix
	}
	if config.RetentionInDays == 0 {
		config.RetentionInDays = -1
	}

	for _, f := range config.Filters {
		err = f.init()
		if err != nil {
			return err
		}
	}

	return nil
}

// Try to parse the timestampFromLogLine value from the log entry line.
// The parser logic will be based on the timestampFromLogLine regex, and time zone info.
// If the parsing operation encounters any issue, int64(0) is returned.
func (config *FileConfig) timestampFromLogLine(logValue string) time.Time {
	if config.TimestampRegexP == nil {
		return time.Time{}
	}
	index := config.TimestampRegexP.FindStringSubmatchIndex(logValue)
	if len(index) > 3 {
		timestampContent := (logValue)[index[2]:index[3]]
		if len(index) > 5 {
			start := index[4] - index[2]
			end := index[5] - index[2]
			//append "000" to 2nd submatch in order to guarantee the fractional second at least has 3 digits
			fracSecond := fmt.Sprintf("%s000", timestampContent[start:end])
			replacement := fmt.Sprintf(".%s", fracSecond[:3])
			timestampContent = fmt.Sprintf("%s%s%s", timestampContent[:start], replacement, timestampContent[end:])
		}
		var err error
		var timestamp time.Time
		for _, timestampLayout := range config.TimestampLayout {
			timestamp, err = time.ParseInLocation(timestampLayout, timestampContent, config.TimezoneLoc)
			if err == nil {
				break
			}
		}
		if err != nil {
			log.Printf("E! Error parsing timestampFromLogLine: %s", err)
			return time.Time{}
		}
		if timestamp.Year() == 0 {
			now := time.Now()
			timestamp = timestamp.AddDate(now.Year(), 0, 0)
			// If now is very early January and we are pushing logs from very late
			// December, there will be a very large number of hours different
			// between the dates. 30 * 24 hours will be sufficient.
			if timestamp.Sub(now) > 30*24*time.Hour {
				timestamp = timestamp.AddDate(-1, 0, 0)
			}
		}
		return timestamp
	}
	return time.Time{}
}

// This method determine whether the line is a start line for multiline log entry.
func (config *FileConfig) isMultilineStart(logValue string) bool {

	if config.MultiLineStartPatternP == nil {
		return false
	}
	return config.MultiLineStartPatternP.MatchString(logValue)
}

func ShouldPublish(logGroupName, logStreamName string, filters []*LogFilter, event logs.LogEvent) bool {
	if len(filters) == 0 {
		return true
	}

	ret := shouldPublishHelper(filters, event)
	droppedCount := 0
	if !ret {
		droppedCount = 1
	}
	profiler.Profiler.AddStats([]string{"logfile", logGroupName, logStreamName, "messages", "dropped"}, float64(droppedCount))

	return ret
}

func shouldPublishHelper(filters []*LogFilter, event logs.LogEvent) bool {
	for _, filter := range filters {
		if !filter.ShouldPublish(event) {
			return false
		}
	}
	return true
}

// The default log group name calculation logic if the log group name is not specified.
// It will use the part before the last dot in the file path, e.g.
// file path: "/tmp/TestLogFile.log.2017-07-11-14" -> log group name: "/tmp/TestLogFile.log"
// file path: "/tmp/TestLogFile.log" -> log group name: "/tmp/TestLogFile"
// Note: the above is default log group behavior, it is always recommended to specify the log group name for each input file pattern
func logGroupName(filePath string) string {
	suffix := filepath.Ext(filePath)
	return strings.TrimSuffix(filePath, suffix)
}
