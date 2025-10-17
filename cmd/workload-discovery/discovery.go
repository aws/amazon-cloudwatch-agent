// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/filter"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/java"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/nvidia"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

type Config struct {
	LogLevel     slog.Level    `json:"log_level"`
	Concurrency  int           `json:"concurrency"`
	Timeout      time.Duration `json:"timeout"`
	FilterConfig filter.Config `json:"filter_config"`
}

type Discoverer struct {
	cfg              Config
	logger           *slog.Logger
	processDetectors []detector.ProcessDetector
	deviceDetectors  []detector.DeviceDetector
	filters          filter.Filters
}

func NewDiscoverer(cfg Config, logger *slog.Logger) *Discoverer {
	filters := filter.FromConfig(logger, cfg.FilterConfig)
	return &Discoverer{
		cfg:    cfg,
		logger: logger,
		processDetectors: []detector.ProcessDetector{
			java.NewDetector(logger, filters.Process.Name),
		},
		deviceDetectors: []detector.DeviceDetector{
			nvidia.NewDetector(logger),
		},
		filters: filters,
	}
}

func (d *Discoverer) Discover(ctx context.Context) error {
	start := time.Now()

	var wg sync.WaitGroup
	var processMd, deviceMd detector.MetadataSlice
	var processErr error

	// Run process detection
	wg.Add(1)
	go func() {
		defer wg.Done()
		processes, err := process.Processes()
		if err != nil {
			processErr = err
			return
		}
		processMd, processErr = d.detectMetadataFromProcesses(ctx, processes)
	}()

	// Run device detection
	wg.Add(1)
	go func() {
		defer wg.Done()
		deviceMd = d.detectMetadataFromDevices()
	}()

	wg.Wait()

	if processErr != nil {
		return processErr
	}

	md := append(processMd, deviceMd...)
	d.logger.Debug("Discovered metadata", "elapsed", time.Since(start))
	if len(md) > 0 {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(md); err != nil {
			return err
		}
	}
	return nil
}

func (d *Discoverer) detectMetadataFromProcesses(ctx context.Context, processes []*process.Process) (detector.MetadataSlice, error) {
	d.logger.Debug(
		"Starting discovery",
		"num_process", len(processes),
		"num_worker", d.cfg.Concurrency,
	)

	jobs := make(chan *process.Process, len(processes)-1)
	results := make(chan *detector.Metadata, d.cfg.Concurrency)

	var workerWg sync.WaitGroup
	for w := 0; w < d.cfg.Concurrency; w++ {
		workerWg.Add(1)
		go d.worker(ctx, jobs, results, &workerWg)
	}

	var collectorWg sync.WaitGroup
	var mds detector.MetadataSlice
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		for r := range results {
			mds = append(mds, r)
		}
	}()

	selfPID, err := convertToPid32(os.Getpid())
	for _, p := range processes {
		// skip the workload discovery process
		if err == nil && p.Pid == selfPID {
			continue
		}
		select {
		case jobs <- p:
		case <-ctx.Done():
			break
		}
	}
	close(jobs)
	workerWg.Wait()
	close(results)
	collectorWg.Wait()

	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return mds, nil
}

func (d *Discoverer) worker(ctx context.Context, jobs <-chan *process.Process, results chan<- *detector.Metadata, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-jobs:
			if !ok {
				return
			}
			cachedProcess := util.NewCachedProcess(util.NewProcessWithPID(p))
			if d.filters.Process.Pre != nil && !d.filters.Process.Pre.ShouldInclude(ctx, cachedProcess) {
				d.logger.Debug("Process skipped due to pre-filter", "pid", cachedProcess.PID())
				continue
			}
			mds, err := d.detectMetadataFromProcess(ctx, cachedProcess)
			if err == nil && mds != nil {
				for _, md := range mds {
					results <- md
				}
			}
		}
	}
}

func (d *Discoverer) detectMetadataFromDevices() detector.MetadataSlice {
	var mds detector.MetadataSlice
	for _, deviceDetector := range d.deviceDetectors {
		md, err := deviceDetector.Detect()
		if err != nil {
			continue
		}
		d.logger.Debug("Detected device", "categories", md.Categories)
		mds = append(mds, md)
	}
	return mds
}

func (d *Discoverer) detectMetadataFromProcess(ctx context.Context, p detector.Process) (detector.MetadataSlice, error) {
	var mds detector.MetadataSlice
	for _, processDetector := range d.processDetectors {
		md, err := processDetector.Detect(ctx, p)
		if err != nil {
			if errors.Is(err, detector.ErrSkipProcess) {
				return nil, detector.ErrSkipProcess
			}
			continue
		}
		d.logger.Debug("Detected supported workload(s) for process", "pid", p.PID(), "categories", md.Categories)
		mds = append(mds, md)
	}
	return mds, nil
}

func buildLogger(level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}
	handler := slog.NewTextHandler(os.Stderr, opts)
	return slog.New(handler)
}

func convertToPid32(pid int) (int32, error) {
	if pid < 0 || pid > math.MaxInt32 {
		return 0, errors.New("pid out of range")
	}
	return int32(pid), nil
}

func main() {
	cfg := Config{
		Concurrency: runtime.NumCPU(),
		LogLevel:    slog.LevelDebug,
		Timeout:     500 * time.Millisecond,
		FilterConfig: filter.Config{
			Process: filter.ProcessConfig{
				MinUptime:    10 * time.Second,
				ExcludeNames: []string{paths.JMXJarName},
			},
		},
	}

	logger := buildLogger(cfg.LogLevel)

	d := NewDiscoverer(cfg, logger)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if err := d.Discover(ctx); err != nil {
		logger.Error("Discovery failed", "error", err)
		os.Exit(1)
	}
}
