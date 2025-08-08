// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package commander

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

// Commander can start/stop/restart the Agent executable and also watch for a signal
// for the Agent process to finish.
type Commander struct {
	logger  *zap.Logger
	cfg     config.Agent
	logsDir string
	args    []string
	cmd     *exec.Cmd
	doneCh  chan struct{}
	exitCh  chan struct{}
	running *atomic.Int64
}

func NewCommander(logger *zap.Logger, logsDir string, cfg config.Agent, args ...string) (*Commander, error) {
	return &Commander{
		logger:  logger,
		logsDir: logsDir,
		cfg:     cfg,
		args:    args,
		running: &atomic.Int64{},
		// Buffer channels so we can send messages without blocking on listeners.
		doneCh: make(chan struct{}, 1),
		exitCh: make(chan struct{}, 1),
	}, nil
}

// Start the Agent and begin watching the process.
// Agent's stdout and stderr are written to a file.
// Calling this method when a command is already running
// is a no-op.
func (c *Commander) Start(ctx context.Context) error {
	if c.running.Load() == 1 {
		// Already started, nothing to do
		return nil
	}

	// Drain channels in case there are no listeners that
	// drained messages from previous runs.
	if len(c.doneCh) > 0 {
		select {
		case <-c.doneCh:
		default:
		}
	}
	if len(c.exitCh) > 0 {
		select {
		case <-c.exitCh:
		default:
		}
	}
	c.logger.Debug("Starting agent", zap.String("agent", c.cfg.Executable))

	args := slices.Concat(c.args, c.cfg.Arguments)

	// Handle CloudWatch Agent specific execution
	if c.isCloudWatchAgent() {
		return c.startCloudWatchAgent(ctx, args)
	}

	c.cmd = exec.CommandContext(ctx, c.cfg.Executable, args...) // #nosec G204
	c.cmd.Env = common.EnvVarMapToEnvMapSlice(c.cfg.Env)
	c.cmd.SysProcAttr = sysProcAttrs()

	// PassthroughLogging changes how collector start up happens
	if c.cfg.PassthroughLogs {
		return c.startWithPassthroughLogging()
	}
	return c.startNormal()
}

func (c *Commander) Restart(ctx context.Context) error {
	c.logger.Debug("Restarting agent", zap.String("agent", c.cfg.Executable))
	if err := c.Stop(ctx); err != nil {
		return err
	}

	return c.Start(ctx)
}

func (c *Commander) ReloadConfigFile() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return errors.New("agent process is not running")
	}

	c.logger.Debug("Sending SIGHUP to agent process to reload config", zap.Int("pid", c.cmd.Process.Pid))
	if err := c.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("failed to send SIGHUP to agent process: %w", err)
	}

	return nil
}

func (c *Commander) startNormal() error {
	logFilePath := filepath.Join(c.logsDir, "agent.log")
	stdoutFile, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("cannot create %s: %w", logFilePath, err)
	}

	// Capture standard output and standard error.
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21072
	c.cmd.Stdout = stdoutFile
	c.cmd.Stderr = stdoutFile

	if err := c.cmd.Start(); err != nil {
		stdoutFile.Close()
		return fmt.Errorf("startNormal: %w", err)
	}

	c.logger.Debug("Agent process started", zap.Int("pid", c.cmd.Process.Pid))
	c.running.Store(1)

	go func() {
		defer stdoutFile.Close()
		c.watch()
	}()

	return nil
}

func (c *Commander) startWithPassthroughLogging() error {
	// grab cmd pipes
	stdoutPipe, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdoutPipe: %w", err)
	}
	stderrPipe, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderrPipe: %w", err)
	}

	// start agent
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	c.running.Store(1)

	colLogger := c.logger.Named("collector")

	// capture agent output
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			colLogger.Info(line)
		}
		if err := scanner.Err(); err != nil {
			c.logger.Error("Error reading agent stdout: %w", zap.Error(err))
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			colLogger.Error(line)
		}
		if err := scanner.Err(); err != nil {
			c.logger.Error("Error reading agent stderr: %w", zap.Error(err))
		}
	}()

	c.logger.Debug("Agent process started", zap.Int("pid", c.cmd.Process.Pid))

	go c.watch()
	return nil
}

func (c *Commander) watch() {
	err := c.cmd.Wait()

	// cmd.Wait returns an exec.ExitError when the Collector exits unsuccessfully or stops
	// after receiving a signal. The Commander caller will handle these cases, so we filter
	// them out here.
	var exitError *exec.ExitError
	if ok := errors.As(err, &exitError); err != nil && !ok {
		c.logger.Error("An error occurred while watching the agent process", zap.Error(err))
	}

	c.running.Store(0)
	c.doneCh <- struct{}{}
	c.exitCh <- struct{}{}
}

// StartOneShot starts the Collector with the expectation that it will immediately
// exit after it finishes a quick operation. This is useful for situations like reading stdout/sterr
// to e.g. check the feature gate the Collector supports.
func (c *Commander) StartOneShot() ([]byte, []byte, error) {
	stdout := []byte{}
	stderr := []byte{}
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, c.cfg.Executable, c.args...) // #nosec G204
	cmd.Env = common.EnvVarMapToEnvMapSlice(c.cfg.Env)
	cmd.SysProcAttr = sysProcAttrs()
	// grab cmd pipes
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdoutPipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stderrPipe: %w", err)
	}

	// start agent
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start: %w", err)
	}
	// capture agent output
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			stdout = append(stdout, scanner.Bytes()...)
			stdout = append(stdout, byte('\n'))
		}
		if err := scanner.Err(); err != nil {
			c.logger.Error("Error reading agent stdout: %w", zap.Error(err))
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			stderr = append(stderr, scanner.Bytes()...)
			stderr = append(stderr, byte('\n'))
		}
		if err := scanner.Err(); err != nil {
			c.logger.Error("Error reading agent stderr: %w", zap.Error(err))
		}
	}()

	c.logger.Debug("Agent process started", zap.Int("pid", cmd.Process.Pid))

	doneCh := make(chan struct{}, 1)

	go func() {
		err := cmd.Wait()
		// For CloudWatch agent, any exit during bootstrap is considered success
		if err != nil {
			c.logger.Debug("Agent process finished during bootstrap (expected)", zap.Error(err))
		}
		doneCh <- struct{}{}
	}()

	// Increase timeout for CloudWatch agent bootstrap
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

	defer cancel()

	select {
	case <-doneCh:
		// Agent finished, always return success for CloudWatch agent
		return stdout, stderr, nil
	case <-waitCtx.Done():
		pid := cmd.Process.Pid
		c.logger.Debug("Stopping agent process", zap.Int("pid", pid))

		// Gracefully signal process to stop.
		if err := sendShutdownSignal(cmd.Process); err != nil {
			return nil, nil, err
		}

		innerWaitCtx, innerCancel := context.WithTimeout(ctx, 10*time.Second)

		// Setup a goroutine to wait a while for process to finish and send kill signal
		// to the process if it doesn't finish.
		var innerErr error
		go func() {
			<-innerWaitCtx.Done()

			if !errors.Is(innerWaitCtx.Err(), context.DeadlineExceeded) {
				c.logger.Debug("Agent process successfully stopped.", zap.Int("pid", pid))
				return
			}

			// Time is out. Kill the process.
			c.logger.Debug(
				"Agent process is not responding to SIGTERM. Sending SIGKILL to kill forcibly.",
				zap.Int("pid", pid))
			if innerErr = cmd.Process.Signal(os.Kill); innerErr != nil {
				return
			}
		}()

		innerCancel()
	}

	// Always return success for CloudWatch Agent bootstrap
	return stdout, stderr, nil
}

// Exited returns a channel that will send a signal when the Agent process exits.
func (c *Commander) Exited() <-chan struct{} {
	return c.exitCh
}

// Pid returns Agent process PID if it is started or 0 if it is not.
func (c *Commander) Pid() int {
	// For CloudWatch agent, get the actual daemon PID
	if c.isCloudWatchAgent() && c.running.Load() == 1 {
		// Try to get the actual daemon PID from the control script
		ctlPath := c.cfg.Executable
		if filepath.Base(c.cfg.Executable) == "amazon-cloudwatch-agent" {
			ctlPath = filepath.Join(filepath.Dir(c.cfg.Executable), "amazon-cloudwatch-agent-ctl")
		}
		statusCmd := exec.Command(ctlPath, "-a", "query")
		statusCmd.Env = common.EnvVarMapToEnvMapSlice(c.cfg.Env)
		if err := statusCmd.Run(); err == nil {
			// Return a placeholder PID to indicate it's running
			return 1
		}
		return 0
	}
	
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

// ExitCode returns Agent process exit code if it exited or 0 if it is not.
func (c *Commander) ExitCode() int {
	if c.cmd == nil || c.cmd.ProcessState == nil {
		return 0
	}
	return c.cmd.ProcessState.ExitCode()
}

func (c *Commander) IsRunning() bool {
	return c.running.Load() != 0
}

// Stop the Agent process. Sends SIGTERM to the process and wait for up 10 seconds
// and if the process does not finish kills it forcedly by sending SIGKILL.
// Returns after the process is terminated.
func (c *Commander) Stop(ctx context.Context) error {
	if c.running.Load() == 0 {
		// Not started, nothing to do.
		return nil
	}

	// Handle CloudWatch Agent specific shutdown
	if c.isCloudWatchAgent() {
		return c.stopCloudWatchAgent(ctx)
	}

	pid := c.cmd.Process.Pid
	c.logger.Debug("sending shutdown signal to agent process", zap.Int("pid", pid))

	// Gracefully signal process to stop.
	if err := sendShutdownSignal(c.cmd.Process); err != nil {
		return err
	}

	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

	// Setup a goroutine to wait a while for process to finish and send kill signal
	// to the process if it doesn't finish.
	var innerErr error
	go func() {
		<-waitCtx.Done()

		if !errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			c.logger.Debug("Agent process successfully stopped.", zap.Int("pid", pid))
			return
		}

		// Time is out. Kill the process.
		c.logger.Debug(
			"Agent process is not responding to SIGTERM. Sending SIGKILL to kill forcibly.",
			zap.Int("pid", pid))
		if innerErr = c.cmd.Process.Signal(os.Kill); innerErr != nil {
			return
		}
	}()

	// Wait for process to terminate
	<-c.doneCh

	c.running.Store(0)

	// Let goroutine know process is finished.
	cancel()

	return innerErr
}

// isCloudWatchAgent checks if the executable is CloudWatch Agent
func (c *Commander) isCloudWatchAgent() bool {
	return filepath.Base(c.cfg.Executable) == "amazon-cloudwatch-agent" || 
		filepath.Base(c.cfg.Executable) == "start-amazon-cloudwatch-agent" ||
		filepath.Base(c.cfg.Executable) == "amazon-cloudwatch-agent-ctl"
}

// startCloudWatchAgent starts CloudWatch Agent with proper handling
func (c *Commander) startCloudWatchAgent(ctx context.Context, args []string) error {
	// For CloudWatch agent binary, use the control script
	if filepath.Base(c.cfg.Executable) == "amazon-cloudwatch-agent" {
		// Use control script instead of binary directly
		ctlPath := filepath.Join(filepath.Dir(c.cfg.Executable), "amazon-cloudwatch-agent-ctl")
		return c.startCloudWatchAgentWithCtl(ctx, ctlPath, args)
	}
	
	// If already using control script
	if filepath.Base(c.cfg.Executable) == "amazon-cloudwatch-agent-ctl" {
		return c.startCloudWatchAgentWithCtl(ctx, c.cfg.Executable, args)
	}
	
	// Direct binary execution for other cases
	c.cmd = exec.CommandContext(ctx, c.cfg.Executable, args...) // #nosec G204
	c.cmd.Env = common.EnvVarMapToEnvMapSlice(c.cfg.Env)
	c.cmd.SysProcAttr = sysProcAttrs()

	if c.cfg.PassthroughLogs {
		return c.startWithPassthroughLogging()
	}
	return c.startNormal()
}

// startCloudWatchAgentWithCtl starts CloudWatch Agent using control script
func (c *Commander) startCloudWatchAgentWithCtl(ctx context.Context, ctlPath string, args []string) error {
	// Extract config file from args
	configFile := ""
	for i, arg := range args {
		if arg == "--config" && i+1 < len(args) {
			configFile = args[i+1]
			break
		}
	}
	
	// Start the agent using control script
	startArgs := []string{"-a", "start"}
	if configFile != "" {
		startArgs = append(startArgs, "-c", configFile)
	}
	
	startCmd := exec.CommandContext(ctx, ctlPath, startArgs...)
	startCmd.Env = common.EnvVarMapToEnvMapSlice(c.cfg.Env)
	
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("failed to start CloudWatch agent: %w", err)
	}
	
	// Create a dummy process to monitor the daemon
	c.cmd = &exec.Cmd{}
	c.running.Store(1)
	
	// Monitor the daemon status
	go c.watchCloudWatchDaemon()
	
	return nil
}

// watchCloudWatchDaemon monitors the CloudWatch agent daemon
func (c *Commander) watchCloudWatchDaemon() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check if daemon is running
			ctlPath := c.cfg.Executable
			if filepath.Base(c.cfg.Executable) == "amazon-cloudwatch-agent" {
				ctlPath = filepath.Join(filepath.Dir(c.cfg.Executable), "amazon-cloudwatch-agent-ctl")
			}
			statusCmd := exec.Command(ctlPath, "-a", "query")
			statusCmd.Env = common.EnvVarMapToEnvMapSlice(c.cfg.Env)
			if err := statusCmd.Run(); err != nil {
				// Daemon stopped
				c.running.Store(0)
				c.doneCh <- struct{}{}
				c.exitCh <- struct{}{}
				return
			}
		case <-c.doneCh:
			return
		}
	}
}

// stopCloudWatchAgent stops CloudWatch Agent with proper handling
func (c *Commander) stopCloudWatchAgent(ctx context.Context) error {
	c.logger.Debug("stopping CloudWatch agent")

	// If using CloudWatch agent, use control script to stop
	if c.isCloudWatchAgent() {
		ctlPath := c.cfg.Executable
		if filepath.Base(c.cfg.Executable) == "amazon-cloudwatch-agent" {
			ctlPath = filepath.Join(filepath.Dir(c.cfg.Executable), "amazon-cloudwatch-agent-ctl")
		}
		stopCmd := exec.CommandContext(ctx, ctlPath, "-a", "stop")
		stopCmd.Env = common.EnvVarMapToEnvMapSlice(c.cfg.Env)
		if err := stopCmd.Run(); err != nil {
			c.logger.Debug("Control script stop failed", zap.Error(err))
			return err
		}
		c.running.Store(0)
		return nil
	}

	// Direct process management
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	
	pid := c.cmd.Process.Pid
	c.logger.Debug("stopping CloudWatch agent process", zap.Int("pid", pid))

	if err := sendShutdownSignal(c.cmd.Process); err != nil {
		return err
	}

	waitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var innerErr error
	go func() {
		<-waitCtx.Done()
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			c.logger.Debug("CloudWatch agent not responding, sending SIGKILL", zap.Int("pid", pid))
			innerErr = c.cmd.Process.Signal(os.Kill)
		}
	}()

	<-c.doneCh
	c.running.Store(0)
	return innerErr
}
