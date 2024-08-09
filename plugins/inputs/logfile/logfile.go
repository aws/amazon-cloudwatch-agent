// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/globpath"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail"
)

type LogFile struct {
	//array of file config for file to be monitored.
	FileConfig []FileConfig `toml:"file_config"`
	//store the offset of file already published.
	FileStateFolder string `toml:"file_state_folder"`
	//destination
	Destination string `toml:"destination"`

	Log telegraf.Logger `toml:"-"`

	configs           map[*FileConfig]map[string]*tailerSrc
	done              chan struct{}
	removeTailerSrcCh chan *tailerSrc
	started           bool
}

func NewLogFile() *LogFile {
	return &LogFile{
		configs:           make(map[*FileConfig]map[string]*tailerSrc),
		done:              make(chan struct{}),
		removeTailerSrcCh: make(chan *tailerSrc, 100),
	}
}

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
      timestamp_layout = ["_2 Jan 2006 15:04:05"]
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

func (t *LogFile) SampleConfig() string {
	return sampleConfig
}

func (t *LogFile) Description() string {
	return "Stream a log file, like the tail -f command"
}

func (t *LogFile) Gather(acc telegraf.Accumulator) error {
	return nil
}

func (t *LogFile) Start(acc telegraf.Accumulator) error {
	// Create the log file state folder.
	err := os.MkdirAll(t.FileStateFolder, 0755)
	if err != nil {
		return fmt.Errorf("failed to create state file directory %s: %v", t.FileStateFolder, err)
	}

	// Clean state file on init and regularly
	go func() {
		t.cleanupStateFolder()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.cleanupStateFolder()
			case <-t.done:
				t.Log.Debugf("Cleanup state folder routine received shutdown signal, stopping.")
				return
			}
		}
	}()

	// Initialize all the file configs
	for i := range t.FileConfig {
		if err := t.FileConfig[i].init(); err != nil {
			return fmt.Errorf("invalid file config init %v with err %v", t.FileConfig[i], err)
		}
	}

	t.started = true
	t.Log.Infof("turned on logs plugin")
	return nil
}

func (t *LogFile) Stop() {
	// Tailer srcs are stopped by log agent after the output plugin is stopped instead of here
	// because the tailersrc would like to record an accurate uploaded offset
	close(t.done)
}

// Try to find if there is any new file needs to be added for monitoring.
func (t *LogFile) FindLogSrc() []logs.LogSrc {
	if !t.started {
		t.Log.Warn("not started with file state folder %s", t.FileStateFolder)
		return nil
	}

	var srcs []logs.LogSrc

	t.cleanUpStoppedTailerSrc()

	// Create a "tailer" for each file
	for i := range t.FileConfig {
		fileconfig := &t.FileConfig[i]
		targetFiles, err := t.getTargetFiles(fileconfig)
		if err != nil {
			t.Log.Errorf("Failed to find target files for file config %v, with error: %v", fileconfig.FilePath, err)
		}
		for _, filename := range targetFiles {
			dests, ok := t.configs[fileconfig]
			if !ok {
				dests = make(map[string]*tailerSrc)
				t.configs[fileconfig] = dests
			}

			if _, ok := dests[filename]; ok {
				continue
			} else if fileconfig.AutoRemoval {
				// This logic means auto_removal does not work with publish_multi_logs
				for _, dst := range dests {
					// Stop all other tailers in favor of the newly found file
					dst.tailer.StopAtEOF()
				}
			}

			var seekFile *tail.SeekInfo
			offset, err := t.restoreState(filename)
			if err == nil { // Missing state file would be an error too
				seekFile = &tail.SeekInfo{Whence: io.SeekStart, Offset: offset}
			} else if !fileconfig.Pipe && !fileconfig.FromBeginning {
				seekFile = &tail.SeekInfo{Whence: io.SeekEnd, Offset: 0}
			}

			isutf16 := false
			if fileconfig.Encoding == "utf-16" || fileconfig.Encoding == "utf-16le" || fileconfig.Encoding == "UTF-16" || fileconfig.Encoding == "UTF-16LE" {
				isutf16 = true
			}

			tailer, err := tail.TailFile(filename,
				tail.Config{
					ReOpen:      false,
					Follow:      true,
					Location:    seekFile,
					MustExist:   true,
					Pipe:        fileconfig.Pipe,
					Poll:        true,
					MaxLineSize: fileconfig.MaxEventSize,
					IsUTF16:     isutf16,
				})

			if err != nil {
				t.Log.Errorf("Failed to tail file %v with error: %v", filename, err)
				continue
			}

			var mlCheck func(string) bool
			if fileconfig.MultiLineStartPattern != "" {
				mlCheck = fileconfig.isMultilineStart
			}

			groupName := fileconfig.LogGroupName
			streamName := fileconfig.LogStreamName

			// In case of multilog, the group and stream has to be generated here
			// since it is based on the actual file name
			if fileconfig.PublishMultiLogs {
				if groupName == "" {
					groupName = generateLogGroupName(filename)
				} else {
					streamName = generateLogStreamName(filename, fileconfig.LogStreamName)
				}
			}

			destination := fileconfig.Destination
			if destination == "" {
				destination = t.Destination
			}

			src := NewTailerSrc(
				groupName, streamName,
				t.Destination,
				t.getStateFilePath(filename),
				fileconfig.LogGroupClass,
				tailer,
				fileconfig.AutoRemoval,
				mlCheck,
				fileconfig.Filters,
				fileconfig.timestampFromLogLine,
				fileconfig.Enc,
				fileconfig.MaxEventSize,
				fileconfig.TruncateSuffix,
				fileconfig.RetentionInDays,
			)

			src.AddCleanUpFn(func(ts *tailerSrc) func() {
				return func() {
					select {
					case <-t.done: // No clean up needed after input plugin is stopped
					case t.removeTailerSrcCh <- ts:
					}

				}
			}(src))

			srcs = append(srcs, src)

			dests[filename] = src
		}
	}

	return srcs
}

func (t *LogFile) getTargetFiles(fileconfig *FileConfig) ([]string, error) {
	filePath := fileconfig.FilePath
	blacklistP := fileconfig.BlacklistRegexP
	g, err := globpath.Compile(filePath)
	if err != nil {
		return nil, fmt.Errorf("file_path glob %s failed to compile, %s", filePath, err)
	}

	var targetFileList []string
	var targetFileName string
	var targetModTime time.Time
	for matchedFileName, matchedFileInfo := range g.Match() {

		// we do not allow customer to monitor the file in t.FileStateFolder, it will monitor all of the state files
		if t.FileStateFolder != "" && strings.HasPrefix(matchedFileName, t.FileStateFolder) {
			continue
		}

		if isCompressedFile(matchedFileName) {
			continue
		}

		// If it's a dir or a symbolic link pointing to a dir, ignore it
		if isDir, err := isDirectory(matchedFileName); err != nil {
			return nil, fmt.Errorf("error tailing file %v with error: %v", matchedFileName, err)
		} else if isDir {
			continue
		}

		// Add another file blacklist here
		fileBaseName := filepath.Base(matchedFileName)
		if blacklistP != nil && blacklistP.MatchString(fileBaseName) {
			continue
		}
		if !fileconfig.PublishMultiLogs {
			if targetFileName == "" || matchedFileInfo.ModTime().After(targetModTime) {
				targetFileName = matchedFileName
				targetModTime = matchedFileInfo.ModTime()
			}
		} else {
			targetFileList = append(targetFileList, matchedFileName)
		}
	}
	//If targetFileName != "", it means customer doesn't enable publish_multi_logs feature, targetFileList should be empty in this case.
	if targetFileName != "" {
		targetFileList = append(targetFileList, targetFileName)
	}

	return targetFileList, nil
}

// The plugin will look at the state folder, and restore the offset of the file seeked if such state exists.
func (t *LogFile) restoreState(filename string) (int64, error) {
	filePath := t.getStateFilePath(filename)

	if _, err := os.Stat(filePath); err != nil {
		t.Log.Debugf("The state file %s for %s does not exist: %v", filePath, filename, err)
		return 0, err
	}

	byteArray, err := os.ReadFile(filePath)
	if err != nil {
		t.Log.Warnf("Issue encountered when reading offset from file %s: %v", filename, err)
		return 0, err
	}

	offset, err := strconv.ParseInt(strings.Split(string(byteArray), "\n")[0], 10, 64)
	if err != nil {
		t.Log.Warnf("Issue encountered when parsing offset value %v: %v", byteArray, err)
		return 0, err
	}

	if offset < 0 {
		return 0, fmt.Errorf("negative state file offset, %v, %v", filePath, offset)
	}
	t.Log.Infof("Reading from offset %v in %s", offset, filename)
	return offset, nil
}

func (t *LogFile) getStateFilePath(filename string) string {
	if t.FileStateFolder == "" {
		return ""
	}

	return filepath.Join(t.FileStateFolder, escapeFilePath(filename))
}

func (t *LogFile) cleanupStateFolder() {
	files, err := filepath.Glob(t.FileStateFolder + string(filepath.Separator) + "*")
	if err != nil {
		t.Log.Errorf("Error happens in cleanup state folder %s: %v", t.FileStateFolder, err)
	}
	for _, file := range files {
		if info, err := os.Stat(file); err != nil || info.IsDir() {
			t.Log.Debugf("File %v does not exist or is a dirctory: %v, %v", file, err, info)
			continue
		}

		if strings.Contains(file, logscommon.WindowsEventLogPrefix) {
			continue
		}

		byteArray, err := os.ReadFile(file)
		if err != nil {
			t.Log.Errorf("Error happens when reading the content from file %s in clean up state fodler step: %v", file, err)
			continue
		}
		contentArray := strings.Split(string(byteArray), "\n")
		if len(contentArray) >= 2 {
			if _, err = os.Stat(contentArray[1]); err == nil {
				// the original source file still exists
				continue
			}
		}
		if err = os.Remove(file); err != nil {
			t.Log.Errorf("Error happens when deleting old state file %s: %v", file, err)
		}
	}
}

func (t *LogFile) cleanUpStoppedTailerSrc() {
	// Clean up stopped tailer sources
	for {
		select {
		case rts := <-t.removeTailerSrcCh:
			for _, dsts := range t.configs {
				for n, ts := range dsts {
					if ts == rts {
						delete(dsts, n)
					}
				}
			}
		default:
			return
		}
	}
}

// Compressed file should be skipped.
// This func is to determine whether the file is compressed or not based on the file name suffix.
func isCompressedFile(filename string) bool {
	suffix := filepath.Ext(filename)
	switch suffix {
	case
		".tar",
		".zst",
		".gz",
		".zip",
		".bz2",
		".rar":
		return true
	}
	return false
}

func escapeFilePath(filePath string) string {
	escapedFilePath := filepath.ToSlash(filePath)
	escapedFilePath = strings.Replace(escapedFilePath, "/", "_", -1)
	escapedFilePath = strings.Replace(escapedFilePath, " ", "_", -1)
	escapedFilePath = strings.Replace(escapedFilePath, ":", "_", -1)
	return escapedFilePath
}

func generateLogGroupName(fileName string) string {
	invalidCharRep := regexp.MustCompile("[^\\.\\-_/#A-Za-z0-9]")
	validChar := "_"
	s := strings.ReplaceAll(fileName, "\\", "/")
	for "" != invalidCharRep.FindString(s) {
		invalidChar := invalidCharRep.FindString(s)
		s = strings.ReplaceAll(s, invalidChar, validChar)
	}

	return s
}

func generateLogStreamName(fileName string, streamName string) string {
	s := strings.ReplaceAll(generateLogGroupName(fileName), "/", "_")
	return fmt.Sprintf("%s_%s", streamName, s)
}

// Directory should be skipped.
// This func is to determine whether the file is actually a directory or a symbolic link pointing to a directory
func isDirectory(filename string) (bool, error) {
	path, err := filepath.EvalSymlinks(filename)
	if err != nil {
		return false, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if info != nil {
		return info.IsDir(), nil
	}
	return false, nil
}

func init() {
	inputs.Add("logfile", func() telegraf.Input {
		return NewLogFile()
	})
}
