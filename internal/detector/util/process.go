// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"context"

	"github.com/shirou/gopsutil/v4/process"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
)

// CachedProcess provides a caching wrapper around the detector.Process. It caches the fields after their first
// retrieval.
type CachedProcess struct {
	process         detector.Process
	exe             string
	errExe          error
	cwd             string
	errCwd          error
	cmdlineSlice    []string
	errCmdlineSlice error
	environ         []string
	errEnviron      error
	createTime      int64
	errCreateTime   error
}

func NewCachedProcess(p detector.Process) detector.Process {
	return &CachedProcess{process: p}
}

func (p *CachedProcess) PID() int32 {
	return p.process.PID()
}

func (p *CachedProcess) ExeWithContext(ctx context.Context) (string, error) {
	if p.exe != "" {
		return p.exe, nil
	}
	if p.errExe != nil {
		return "", p.errExe
	}
	p.exe, p.errExe = p.process.ExeWithContext(ctx)
	return p.exe, p.errExe
}

func (p *CachedProcess) CwdWithContext(ctx context.Context) (string, error) {
	if p.cwd != "" {
		return p.cwd, nil
	}
	if p.errCwd != nil {
		return "", p.errCwd
	}
	p.cwd, p.errCwd = p.process.CwdWithContext(ctx)
	return p.cwd, p.errCwd
}

func (p *CachedProcess) CmdlineSliceWithContext(ctx context.Context) ([]string, error) {
	if len(p.cmdlineSlice) != 0 {
		return p.cmdlineSlice, nil
	}
	if p.errCmdlineSlice != nil {
		return nil, p.errCmdlineSlice
	}
	p.cmdlineSlice, p.errCmdlineSlice = p.process.CmdlineSliceWithContext(ctx)
	return p.cmdlineSlice, p.errCmdlineSlice
}

func (p *CachedProcess) EnvironWithContext(ctx context.Context) ([]string, error) {
	if len(p.environ) != 0 {
		return p.environ, nil
	}
	if p.errEnviron != nil {
		return nil, p.errEnviron
	}
	p.environ, p.errEnviron = p.process.EnvironWithContext(ctx)
	return p.environ, p.errEnviron
}

func (p *CachedProcess) CreateTimeWithContext(ctx context.Context) (int64, error) {
	if p.createTime != 0 {
		return p.createTime, nil
	}
	if p.errCreateTime != nil {
		return 0, p.errCreateTime
	}
	p.createTime, p.errCreateTime = p.process.CreateTimeWithContext(ctx)
	return p.createTime, p.errCreateTime
}

// ProcessWithPID provides a wrapper for the gopsutil process.Process to expose the PID.
type ProcessWithPID struct {
	*process.Process
}

func NewProcessWithPID(process *process.Process) *ProcessWithPID {
	return &ProcessWithPID{Process: process}
}

func (p *ProcessWithPID) PID() int32 {
	return p.Pid
}
