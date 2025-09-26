// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/java"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
)

type Config struct {
	LogLevel    slog.Level    `json:"log_level"`
	Concurrency int           `json:"concurrency"`
	Timeout     time.Duration `json:"timeout"`
}

type Discoverer struct {
	cfg              Config
	logger           *slog.Logger
	processDetectors []detector.ProcessDetector
}

func NewDiscoverer(cfg Config, logger *slog.Logger) *Discoverer {
	return &Discoverer{
		cfg:    cfg,
		logger: logger,
		processDetectors: []detector.ProcessDetector{
			java.NewDetector(logger),
		},
	}
}

func (d *Discoverer) Discover(ctx context.Context) error {
	start := time.Now()
	processes, err := process.Processes()
	if err != nil {
		return err
	}
	md, err := d.detectMetadataFromProcesses(ctx, processes)
	if err != nil {
		return err
	}
	d.logger.Debug("Discovered metadata", "elapsed", time.Since(start))
	if len(md) > 0 {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err = encoder.Encode(md); err != nil {
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

	selfPID := int32(os.Getpid()) // nolint:gosec
	for _, p := range processes {
		// skip the workload discovery process
		if p.Pid == selfPID {
			continue
		}
		select {
		case jobs <- p:
		case <-ctx.Done():
			close(jobs)
			workerWg.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)
	workerWg.Wait()
	close(results)
	collectorWg.Wait()

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
			mds, err := d.detectMetadataFromProcess(ctx, util.NewCachedProcess(util.NewProcessWithPID(p)))
			if err == nil && mds != nil {
				for _, md := range mds {
					results <- md
				}
			}
		}
	}
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

func main() {
	cfg := Config{
		Concurrency: runtime.NumCPU(),
		LogLevel:    slog.LevelDebug,
		Timeout:     100 * time.Millisecond,
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
