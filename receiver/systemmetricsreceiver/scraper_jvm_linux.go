//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	jvmScrapeTimeout = 5 * time.Second
	jvmMaxResponse   = 131072 // 128 KB
	jvmScrapeCommand = "GET /metrics"

	// Socket discovery
	jvmSocketPrefix = "@aws-jvm-metrics-"
	sockDgram       = "0002"
	procNetUnix     = "/proc/net/unix"
	maxJVMSockets   = 100

	// Metric names from the socket
	jvmHeapMax       = "jvm_heap_max_bytes"
	jvmHeapCommitted = "jvm_heap_committed_bytes"
	jvmHeapAfterGC   = "jvm_heap_after_gc_bytes"

	// Published metric names
	metricHeapMax       = "heap_max_bytes"
	metricHeapCommitted = "heap_committed_bytes"
	metricHeapAfterGC   = "heap_after_gc_bytes"
	metricHeapFree      = "heap_free_after_gc_bytes"
	metricHeapUtilized  = "aggregate_heap_after_gc_utilized"
	metricAggHeapMax    = "aggregate_heap_max_bytes"
	metricAggHeapFree   = "aggregate_heap_free_after_gc_bytes"
	metricAggJVMCount   = "aggregate_jvm_count"
)

var pidRegex = regexp.MustCompile(`^\d+$`)

// discoveredSocket represents a Java agent socket found via /proc/net/unix.
type discoveredSocket struct {
	pid  string
	addr string // abstract socket address with \x00 prefix
}

type jvmScraper struct {
	logger          *zap.Logger
	seq             uint64
	procNetUnixPath string
}

func newJVMScraper(logger *zap.Logger) *jvmScraper {
	return &jvmScraper{logger: logger, procNetUnixPath: procNetUnix}
}

func (s *jvmScraper) Name() string {
	return "jvm"
}

// heapData holds extracted heap values for a single JVM.
type heapData struct {
	pid            string
	maxBytes       float64
	committedBytes float64
	usedBytes      float64
}

func (s *jvmScraper) Scrape(_ context.Context, metrics pmetric.Metrics) error {
	sockets := s.discoverSockets()
	if len(sockets) == 0 {
		return nil
	}

	var allHeap []heapData
	now := pcommon.NewTimestampFromTime(time.Now())

	for _, sock := range sockets {
		data, err := s.scrapeSocket(sock.addr)
		if err != nil {
			s.logger.Debug("Failed to scrape JVM socket", zap.String("pid", sock.pid), zap.Error(err))
			continue
		}
		hd, err := s.parseHeap(data, sock.pid)
		if err != nil {
			s.logger.Warn("Failed to parse JVM metrics", zap.String("pid", sock.pid), zap.Error(err))
			continue
		}
		if hd == nil {
			continue
		}
		allHeap = append(allHeap, *hd)
		s.emitPerJVM(metrics, *hd, now)
	}

	s.emitAggregate(metrics, allHeap, now)
	return nil
}

// parseHeap extracts heap max, committed, and after-GC bytes from metric text.
// All three are optional — we emit whatever is available.
// Returns nil only if none of the heap metrics are present.
func (s *jvmScraper) parseHeap(data []byte, pid string) (*heapData, error) {
	metrics := parseMetricsText(data)
	hd := &heapData{pid: pid, maxBytes: -1, committedBytes: -1, usedBytes: -1}

	if v, ok := metrics[jvmHeapMax]; ok {
		hd.maxBytes = v
	}
	if v, ok := metrics[jvmHeapCommitted]; ok {
		hd.committedBytes = v
	}
	if v, ok := metrics[jvmHeapAfterGC]; ok {
		hd.usedBytes = v
	}

	if hd.maxBytes < 0 && hd.committedBytes < 0 && hd.usedBytes < 0 {
		return nil, nil
	}
	return hd, nil
}

// parseMetricsText parses flat "name value" text into metric name → value.
// Skips # comments, empty lines, and lines that fail to parse.
func parseMetricsText(data []byte) map[string]float64 {
	metrics := make(map[string]float64)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(fields[1], 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		metrics[fields[0]] = v
	}
	return metrics
}

// emitPerJVM adds available heap metrics for one JVM. Each metric is independent.
func (s *jvmScraper) emitPerJVM(metrics pmetric.Metrics, hd heapData, now pcommon.Timestamp) {
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	if hd.maxBytes >= 0 {
		addGaugeDP(sm.Metrics().AppendEmpty(), metricHeapMax, "Bytes", hd.maxBytes, now)
	}
	if hd.committedBytes >= 0 {
		addGaugeDP(sm.Metrics().AppendEmpty(), metricHeapCommitted, "Bytes", hd.committedBytes, now)
	}
	if hd.usedBytes >= 0 {
		addGaugeDP(sm.Metrics().AppendEmpty(), metricHeapAfterGC, "Bytes", hd.usedBytes, now)
	}
	if hd.maxBytes >= 0 && hd.usedBytes >= 0 {
		addGaugeDP(sm.Metrics().AppendEmpty(), metricHeapFree, "Bytes", hd.maxBytes-hd.usedBytes, now)
	}
}

// emitAggregate adds per-box aggregate heap metrics across all JVMs.
func (s *jvmScraper) emitAggregate(metrics pmetric.Metrics, allHeap []heapData, now pcommon.Timestamp) {
	if len(allHeap) == 0 {
		return
	}

	var totalMax, totalUsed, totalFree float64
	var hasMax, hasUtilized bool
	for _, hd := range allHeap {
		if hd.maxBytes >= 0 {
			totalMax += hd.maxBytes
			hasMax = true
		}
		if hd.maxBytes >= 0 && hd.usedBytes >= 0 {
			totalUsed += hd.usedBytes
			totalFree += hd.maxBytes - hd.usedBytes
			hasUtilized = true
		}
	}

	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	addGaugeDP(sm.Metrics().AppendEmpty(), metricAggJVMCount, "Count", float64(len(allHeap)), now)
	if hasMax {
		addGaugeDP(sm.Metrics().AppendEmpty(), metricAggHeapMax, "Bytes", totalMax, now)
	}
	if hasUtilized {
		addGaugeDP(sm.Metrics().AppendEmpty(), metricAggHeapFree, "Bytes", totalFree, now)
	}
	if hasUtilized && totalMax > 0 {
		addGaugeDP(sm.Metrics().AppendEmpty(), metricHeapUtilized, "Percent", totalUsed/totalMax*100, now)
	}
}

func (s *jvmScraper) discoverSockets() []discoveredSocket {
	f, err := os.Open(s.procNetUnixPath)
	if err != nil {
		s.logger.Debug("Failed to read /proc/net/unix", zap.Error(err))
		return nil
	}
	defer f.Close()

	var sockets []discoveredSocket
	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 {
			continue
		}
		if fields[4] != sockDgram {
			continue
		}
		path := fields[len(fields)-1]
		if !strings.HasPrefix(path, jvmSocketPrefix) {
			continue
		}
		pid := path[len(jvmSocketPrefix):]
		if !pidRegex.MatchString(pid) {
			continue
		}
		sockets = append(sockets, discoveredSocket{
			pid:  pid,
			addr: "\x00" + path[1:],
		})
		if len(sockets) >= maxJVMSockets {
			s.logger.Warn("Socket discovery cap reached", zap.Int("max", maxJVMSockets))
			break
		}
	}
	if err := scanner.Err(); err != nil {
		s.logger.Warn("Error reading /proc/net/unix", zap.Error(err))
	}
	return sockets
}

func (s *jvmScraper) scrapeSocket(serverAddr string) ([]byte, error) {
	s.seq++
	clientAddr := fmt.Sprintf("\x00cwagent-scraper-%d", s.seq)

	local := &net.UnixAddr{Name: clientAddr, Net: "unixgram"}
	remote := &net.UnixAddr{Name: serverAddr, Net: "unixgram"}

	conn, err := net.DialUnix("unixgram", local, remote)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(jvmScrapeTimeout)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	if _, err := conn.Write([]byte(jvmScrapeCommand)); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	buf := make([]byte, jvmMaxResponse)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("recv: %w", err)
	}
	return buf[:n], nil
}
