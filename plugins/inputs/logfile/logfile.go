// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/state"
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
	//maximum number of distinct, non-overlapping offset ranges to store.
	MaxPersistState int `toml:"max_persist_state"`

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
      trim_timestamp = false
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
	t.Log.Infof("[LOGFILE START] Initializing LogFile plugin")
	t.Log.Infof("[LOGFILE START] State folder: %s", t.FileStateFolder)
	t.Log.Infof("[LOGFILE START] Number of file configs: %d", len(t.FileConfig))
	t.Log.Infof("[LOGFILE START] Default destination: %s", t.Destination)
	t.Log.Infof("[LOGFILE START] Max persist state: %d", t.MaxPersistState)
	
	// Create the log file state folder.
	err := os.MkdirAll(t.FileStateFolder, 0755)
	if err != nil {
		t.Log.Errorf("[LOGFILE START] Failed to create state file directory %s: %v", t.FileStateFolder, err)
		return fmt.Errorf("failed to create state file directory %s: %v", t.FileStateFolder, err)
	}
	t.Log.Infof("[LOGFILE START] Successfully created state file directory: %s", t.FileStateFolder)

	// Clean state file on init and regularly
	go func() {
		t.Log.Debugf("[LOGFILE CLEANUP] Starting cleanup routine")
		t.cleanupStateFolder()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.Log.Debugf("[LOGFILE CLEANUP] Running scheduled cleanup")
				t.cleanupStateFolder()
			case <-t.done:
				t.Log.Debugf("[LOGFILE CLEANUP] Cleanup state folder routine received shutdown signal, stopping.")
				return
			}
		}
	}()

	// Initialize all the file configs
	for i := range t.FileConfig {
		t.Log.Infof("[LOGFILE START] Initializing file config %d:", i)
		t.Log.Infof("[LOGFILE START]   - File path: %s", t.FileConfig[i].FilePath)
		t.Log.Infof("[LOGFILE START]   - Log group: %s", t.FileConfig[i].LogGroupName)
		t.Log.Infof("[LOGFILE START]   - Log stream: %s", t.FileConfig[i].LogStreamName)
		t.Log.Infof("[LOGFILE START]   - Max event size: %d bytes", t.FileConfig[i].MaxEventSize)
		t.Log.Infof("[LOGFILE START]   - From beginning: %t", t.FileConfig[i].FromBeginning)
		t.Log.Infof("[LOGFILE START]   - Multi logs: %t", t.FileConfig[i].PublishMultiLogs)
		t.Log.Infof("[LOGFILE START]   - Auto removal: %t", t.FileConfig[i].AutoRemoval)
		t.Log.Infof("[LOGFILE START]   - Encoding: %s", t.FileConfig[i].Encoding)
		
		if err := t.FileConfig[i].init(); err != nil {
			t.Log.Errorf("[LOGFILE START] Invalid file config init for config %d: %v, error: %v", i, t.FileConfig[i], err)
			return fmt.Errorf("invalid file config init %v with err %v", t.FileConfig[i], err)
		}
		t.Log.Infof("[LOGFILE START] Successfully initialized file config %d", i)
	}

	t.started = true
	t.Log.Infof("[LOGFILE START] Successfully turned on logs plugin with %d file configurations", len(t.FileConfig))
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
		t.Log.Warnf("[LOGFILE FIND] Plugin not started with file state folder %s", t.FileStateFolder)
		return nil
	}

	t.Log.Debugf("[LOGFILE FIND] Starting log source discovery")
	var srcs []logs.LogSrc

	t.cleanUpStoppedTailerSrc()

	es := entitystore.GetEntityStore()

	// Create a "tailer" for each file
	for i := range t.FileConfig {
		fileconfig := &t.FileConfig[i]
		t.Log.Debugf("[LOGFILE FIND] Processing file config %d: %s", i, fileconfig.FilePath)

		//Add file -> {serviceName,  deploymentEnvironment} mapping to entity store
		if es != nil {
			es.AddServiceAttrEntryForLogFile(entitystore.LogFileGlob(fileconfig.FilePath), fileconfig.ServiceName, fileconfig.Environment)
			t.Log.Debugf("[LOGFILE FIND] Added service attributes to entity store for %s", fileconfig.FilePath)
		}

		targetFiles, err := t.getTargetFiles(fileconfig)
		if err != nil {
			t.Log.Errorf("[LOGFILE FIND] Failed to find target files for file config %v, with error: %v", fileconfig.FilePath, err)
			continue
		}
		
		t.Log.Infof("[LOGFILE FIND] Found %d target files for pattern %s", len(targetFiles), fileconfig.FilePath)
		for idx, filename := range targetFiles {
			t.Log.Debugf("[LOGFILE FIND] Processing target file %d/%d: %s", idx+1, len(targetFiles), filename)
			
			dests, ok := t.configs[fileconfig]
			if !ok {
				dests = make(map[string]*tailerSrc)
				t.configs[fileconfig] = dests
				t.Log.Debugf("[LOGFILE FIND] Created new destination map for file config")
			}

			if _, ok := dests[filename]; ok {
				t.Log.Debugf("[LOGFILE FIND] File %s already being monitored, skipping", filename)
				continue
			} else if fileconfig.AutoRemoval {
				// This logic means auto_removal does not work with publish_multi_logs
				t.Log.Infof("[LOGFILE FIND] Auto removal enabled, stopping existing tailers for %s", filename)
				for existingFile, dst := range dests {
					t.Log.Debugf("[LOGFILE FIND] Stopping tailer for existing file: %s", existingFile)
					// Stop all other tailers in favor of the newly found file
					dst.tailer.StopAtEOF()
				}
			}

			// Check file size and properties
			if fileInfo, err := os.Stat(filename); err == nil {
				t.Log.Infof("[LOGFILE FIND] File stats for %s:", filename)
				t.Log.Infof("[LOGFILE FIND]   - Size: %d bytes (%.2f KB)", fileInfo.Size(), float64(fileInfo.Size())/1024)
				t.Log.Infof("[LOGFILE FIND]   - Modified: %s", fileInfo.ModTime().Format(time.RFC3339))
				t.Log.Infof("[LOGFILE FIND]   - Mode: %s", fileInfo.Mode())
			}

			stateManager := state.NewFileRangeManager(state.ManagerConfig{
				StateFileDir:      t.FileStateFolder,
				Name:              filename,
				MaxPersistedItems: max(1, t.MaxPersistState),
			})
			t.Log.Debugf("[LOGFILE FIND] Created state manager for %s", filename)

			var seekFile *tail.SeekInfo
			restored, err := stateManager.Restore()
			if err == nil { // Missing state file would be an error too
				seekFile = &tail.SeekInfo{Whence: io.SeekStart, Offset: restored.Last().EndOffsetInt64()}
				t.Log.Infof("[LOGFILE FIND] Restored state for %s, seeking to offset %d", filename, restored.Last().EndOffsetInt64())
			} else if !fileconfig.Pipe && !fileconfig.FromBeginning {
				seekFile = &tail.SeekInfo{Whence: io.SeekEnd, Offset: 0}
				t.Log.Infof("[LOGFILE FIND] No state found for %s, seeking to end of file", filename)
			} else {
				t.Log.Infof("[LOGFILE FIND] Starting from beginning for %s (pipe: %t, from_beginning: %t)", filename, fileconfig.Pipe, fileconfig.FromBeginning)
			}

			var gapsToRead state.RangeList
			if !restored.OnlyUseMaxOffset() {
				gapsToRead = state.InvertRanges(restored)
				t.Log.Debugf("[LOGFILE FIND] Found %d gaps to read for %s", len(gapsToRead), filename)
			}
			
			isutf16 := false
			if fileconfig.Encoding == "utf-16" || fileconfig.Encoding == "utf-16le" || fileconfig.Encoding == "UTF-16" || fileconfig.Encoding == "UTF-16LE" {
				isutf16 = true
				t.Log.Debugf("[LOGFILE FIND] UTF-16 encoding detected for %s", filename)
			}

			t.Log.Infof("[LOGFILE FIND] Creating tailer for %s with max line size %d bytes", filename, fileconfig.MaxEventSize)
			tailer, err := tail.TailFile(filename,
				tail.Config{
					ReOpen:      false,
					Follow:      true,
					Location:    seekFile,
					GapsToRead:  gapsToRead,
					MustExist:   true,
					Pipe:        fileconfig.Pipe,
					Poll:        true,
					MaxLineSize: fileconfig.MaxEventSize,
					IsUTF16:     isutf16,
				})

			if err != nil {
				t.Log.Errorf("[LOGFILE FIND] Failed to tail file %v with error: %v", filename, err)
				continue
			}
			t.Log.Infof("[LOGFILE FIND] Successfully created tailer for %s", filename)

			var mlCheck func(string) bool
			if fileconfig.MultiLineStartPattern != "" {
				mlCheck = fileconfig.isMultilineStart
				t.Log.Debugf("[LOGFILE FIND] Multiline pattern configured for %s: %s", filename, fileconfig.MultiLineStartPattern)
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
				t.Log.Infof("[LOGFILE FIND] Multi-log mode - generated names for %s:", filename)
				t.Log.Infof("[LOGFILE FIND]   - Log Group: %s", groupName)
				t.Log.Infof("[LOGFILE FIND]   - Log Stream: %s", streamName)
			}

			destination := fileconfig.Destination
			if destination == "" {
				destination = t.Destination
			}

			t.Log.Infof("[LOGFILE FIND] Creating tailer source for %s:", filename)
			t.Log.Infof("[LOGFILE FIND]   - Log Group: %s", groupName)
			t.Log.Infof("[LOGFILE FIND]   - Log Stream: %s", streamName)
			t.Log.Infof("[LOGFILE FIND]   - Destination: %s", destination)
			t.Log.Infof("[LOGFILE FIND]   - Max Event Size: %d bytes", fileconfig.MaxEventSize)
			t.Log.Infof("[LOGFILE FIND]   - Truncate Suffix: %s", fileconfig.TruncateSuffix)
			t.Log.Infof("[LOGFILE FIND]   - Retention: %d days", fileconfig.RetentionInDays)

			src := NewTailerSrc(
				groupName, streamName,
				t.Destination,
				stateManager,
				fileconfig.LogGroupClass,
				fileconfig.FilePath,
				tailer,
				fileconfig.AutoRemoval,
				mlCheck,
				fileconfig.Filters,
				fileconfig.timestampFromLogLine,
				fileconfig.Enc,
				fileconfig.MaxEventSize,
				fileconfig.TruncateSuffix,
				fileconfig.RetentionInDays,
				fileconfig.BackpressureMode,
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
			
			t.Log.Infof("[LOGFILE FIND] Successfully created and registered tailer source for %s", filename)
		}
	}

	t.Log.Infof("[LOGFILE FIND] Discovery complete, found %d log sources", len(srcs))
	return srcs
}

func (t *LogFile) getTargetFiles(fileconfig *FileConfig) ([]string, error) {
	filePath := fileconfig.FilePath
	blacklistP := fileconfig.BlacklistRegexP
	
	t.Log.Infof("[LOGFILE TARGET] Starting target file discovery for pattern: %s", filePath)
	t.Log.Debugf("[LOGFILE TARGET] File config details:")
	t.Log.Debugf("[LOGFILE TARGET]   - Multi-log mode: %t", fileconfig.PublishMultiLogs)
	t.Log.Debugf("[LOGFILE TARGET]   - State folder: %s", t.FileStateFolder)
	
	if blacklistP != nil {
		t.Log.Debugf("[LOGFILE TARGET] Blacklist pattern configured: %s", blacklistP.String())
	} else {
		t.Log.Debugf("[LOGFILE TARGET] No blacklist pattern configured")
	}
	
	t.Log.Debugf("[LOGFILE TARGET] Compiling glob pattern: %s", filePath)
	g, err := globpath.Compile(filePath)
	if err != nil {
		t.Log.Errorf("[LOGFILE TARGET] Failed to compile glob pattern %s: %s", filePath, err)
		return nil, fmt.Errorf("file_path glob %s failed to compile, %s", filePath, err)
	}
	t.Log.Debugf("[LOGFILE TARGET] Successfully compiled glob pattern")

	var targetFileList []string
	var targetFileName string
	var targetModTime time.Time
	matchCount := 0
	
	for matchedFileName, matchedFileInfo := range g.Match() {
		matchCount++
		t.Log.Debugf("[LOGFILE TARGET] Evaluating matched file %d: %s", matchCount, matchedFileName)
		
		if t.FileStateFolder != "" && strings.HasPrefix(matchedFileName, t.FileStateFolder) {
			t.Log.Debugf("[LOGFILE TARGET] Skipping file in state folder: %s", matchedFileName)
			continue
		}

		if isCompressedFile(matchedFileName) {
			t.Log.Debugf("[LOGFILE TARGET] Skipping compressed file: %s", matchedFileName)
			continue
		}

		// If it's a dir or a symbolic link pointing to a dir, ignore it
		if isDir, err := isDirectory(matchedFileName); err != nil {
			t.Log.Errorf("[LOGFILE TARGET] Error checking if %s is directory: %v", matchedFileName, err)
			return nil, fmt.Errorf("error tailing file %v with error: %v", matchedFileName, err)
		} else if isDir {
			t.Log.Debugf("[LOGFILE TARGET] Skipping directory: %s", matchedFileName)
			continue
		}

		fileBaseName := filepath.Base(matchedFileName)
		if blacklistP != nil && blacklistP.MatchString(fileBaseName) {
			t.Log.Debugf("[LOGFILE TARGET] Skipping blacklisted file: %s (matches pattern)", matchedFileName)
			continue
		}
		
		// Log file details
		t.Log.Debugf("[LOGFILE TARGET] File details for %s:", matchedFileName)
		t.Log.Debugf("[LOGFILE TARGET]   - Size: %d bytes", matchedFileInfo.Size())
		t.Log.Debugf("[LOGFILE TARGET]   - Modified: %s", matchedFileInfo.ModTime().Format(time.RFC3339))
		t.Log.Debugf("[LOGFILE TARGET]   - Mode: %s", matchedFileInfo.Mode())
		
		if !fileconfig.PublishMultiLogs {
			if targetFileName == "" || matchedFileInfo.ModTime().After(targetModTime) {
				if targetFileName != "" {
					t.Log.Debugf("[LOGFILE TARGET] Replacing previous target %s (older: %s) with %s (newer: %s)", 
						targetFileName, targetModTime.Format(time.RFC3339), 
						matchedFileName, matchedFileInfo.ModTime().Format(time.RFC3339))
				}
				targetFileName = matchedFileName
				targetModTime = matchedFileInfo.ModTime()
			}
		} else {
			targetFileList = append(targetFileList, matchedFileName)
			t.Log.Debugf("[LOGFILE TARGET] Multi-log mode - added file: %s", matchedFileName)
		}
	}
	
	//If targetFileName != "", it means customer doesn't enable publish_multi_logs feature, targetFileList should be empty in this case.
	if targetFileName != "" {
		targetFileList = append(targetFileList, targetFileName)
		t.Log.Infof("[LOGFILE TARGET] Single file mode - selected most recent file: %s (modified: %s)", 
			targetFileName, targetModTime.Format(time.RFC3339))
	}

	t.Log.Infof("[LOGFILE TARGET] Pattern %s matched %d files, selected %d files for monitoring", 
		filePath, matchCount, len(targetFileList))
	
	for i, file := range targetFileList {
		t.Log.Infof("[LOGFILE TARGET] Selected file %d: %s", i+1, file)
	}

	return targetFileList, nil
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
