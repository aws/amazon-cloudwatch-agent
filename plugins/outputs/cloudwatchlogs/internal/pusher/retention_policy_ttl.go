// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
)

const (
	ttlTime = 5 * time.Minute
)

type payload struct {
	group     string
	timestamp time.Time
}

type RetentionPolicyTTL struct {
	logger        telegraf.Logger
	stateFilePath string
	// oldTimestamps come from the TTL file on agent start. Key is escaped group name
	oldTimestamps map[string]time.Time
	// newTimestamps are the new TTLs that will be saved periodically and when the agent is done. Key is escaped group name
	newTimestamps map[string]time.Time
	mu            sync.RWMutex
	ch            chan payload
	done          chan struct{}
}

func NewRetentionPolicyTTL(logger telegraf.Logger, fileStatePath string) *RetentionPolicyTTL {
	r := &RetentionPolicyTTL{
		logger:        logger,
		stateFilePath: filepath.Join(fileStatePath, logscommon.RetentionPolicyTTLFileName),
		oldTimestamps: make(map[string]time.Time),
		newTimestamps: make(map[string]time.Time),
		ch:            make(chan payload, retentionChannelSize),
		done:          make(chan struct{}),
	}

	r.loadTTLState()
	go r.process()
	return r
}

// Update will update the newTimestamps to the current time that will later be persisted to disk.
func (r *RetentionPolicyTTL) Update(group string) {
	r.ch <- payload{
		group:     group,
		timestamp: time.Now(),
	}
}

func (r *RetentionPolicyTTL) Done() {
	close(r.done)
}

// IsExpired checks from the timestamps in the read state file at the agent start.
func (r *RetentionPolicyTTL) IsExpired(group string) bool {
	if ts, ok := r.oldTimestamps[escapeLogGroup(group)]; ok {
		return ts.Add(ttlTime).Before(time.Now())
	}
	// Log group was not in state file -- default to expired
	return true
}

// UpdateFromFile updates the newTimestamps cache using the timestamp from the loaded state file.
func (r *RetentionPolicyTTL) UpdateFromFile(group string) {
	if oldTs, ok := r.oldTimestamps[escapeLogGroup(group)]; ok {
		r.ch <- payload{
			group:     group,
			timestamp: oldTs,
		}
	}
}

func (r *RetentionPolicyTTL) loadTTLState() {
	if _, err := os.Stat(r.stateFilePath); err != nil {
		r.logger.Debug("retention policy ttl state file does not exist")
		return
	}

	file, err := os.Open(r.stateFilePath)
	if err != nil {
		r.logger.Errorf("unable to open retention policy ttl state file: %v", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		split := strings.Split(line, ":")
		if len(split) < 2 {
			r.logger.Errorf("invalid format in retention policy ttl state file: %s", line)
			continue
		}

		group := split[0]
		timestamp, err := strconv.ParseInt(split[1], 10, 64)
		if err != nil {
			r.logger.Errorf("unable to parse timestamp in retention policy ttl for group %s: %v", group, err)
			continue
		}
		r.oldTimestamps[group] = time.UnixMilli(timestamp)
	}

	if err := scanner.Err(); err != nil {
		r.logger.Errorf("error when parsing retention policy ttl state file: %v", err)
		return
	}
}

func (r *RetentionPolicyTTL) process() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()

	for {
		select {
		case payload := <-r.ch:
			r.updateTimestamp(payload.group, payload.timestamp)
		case <-t.C:
			r.saveTTLState()
		case <-r.done:
			r.saveTTLState()
			return
		}
	}
}

func (r *RetentionPolicyTTL) updateTimestamp(group string, timestamp time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.newTimestamps[escapeLogGroup(group)] = timestamp
}

func (r *RetentionPolicyTTL) saveTTLState() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var buf bytes.Buffer
	for group, timestamp := range r.newTimestamps {
		buf.Write([]byte(group + ":" + strconv.FormatInt(timestamp.UnixMilli(), 10) + "\n"))
	}

	err := os.WriteFile(r.stateFilePath, buf.Bytes(), 0644) // nolint:gosec
	if err != nil {
		r.logger.Errorf("unable to write retention policy ttl state file: %v", err)
	}
}

func escapeLogGroup(group string) string {
	escapedLogGroup := filepath.ToSlash(group)
	escapedLogGroup = strings.Replace(escapedLogGroup, "/", "_", -1)
	escapedLogGroup = strings.Replace(escapedLogGroup, " ", "_", -1)
	escapedLogGroup = strings.Replace(escapedLogGroup, ":", "_", -1)
	return escapedLogGroup
}
