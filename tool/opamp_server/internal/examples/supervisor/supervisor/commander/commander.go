package commander

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/internal/examples/supervisor/supervisor/config"
)

// Commander can start/stop/restat the Agent executable and also watch for a signal
// for the Agent process to finish.
type Commander struct {
	logger  types.Logger
	cfg     *config.Agent
	args    []string
	cmd     *exec.Cmd
	doneCh  chan struct{}
	waitCh  chan struct{}
	running int64

	// Process should not be started while being stopped.
	startStopMutex sync.RWMutex
}

func NewCommander(logger types.Logger, cfg *config.Agent, args ...string) (*Commander, error) {
	if cfg.Executable == "" {
		return nil, errors.New("agent.executable config option must be specified")
	}

	return &Commander{
		logger: logger,
		cfg:    cfg,
		args:   args,
	}, nil
}

// Start the Agent and begin watching the process.
// Agent's stdout and stderr are written to a file.
func (c *Commander) Start(ctx context.Context) error {
	c.startStopMutex.Lock()
	defer c.startStopMutex.Unlock()

	c.logger.Debugf(ctx, "Starting agent %s", c.cfg.Executable)

	logFilePath := "agent.log"
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("cannot create %s: %s", logFilePath, err.Error())
	}

	c.cmd = exec.CommandContext(ctx, c.cfg.Executable, c.args...)

	// Capture standard output and standard error.
	c.cmd.Stdout = logFile
	c.cmd.Stderr = logFile

	c.doneCh = make(chan struct{}, 1)
	c.waitCh = make(chan struct{})

	if err := c.cmd.Start(); err != nil {
		return err
	}

	c.logger.Debugf(ctx, "Agent process started, PID=%d", c.cmd.Process.Pid)
	atomic.StoreInt64(&c.running, 1)

	go c.watch()

	return nil
}

func (c *Commander) Restart(ctx context.Context) error {
	if err := c.Stop(ctx); err != nil {
		return err
	}
	if err := c.Start(ctx); err != nil {
		return err
	}
	return nil
}

func (c *Commander) watch() {
	c.cmd.Wait()
	c.doneCh <- struct{}{}
	atomic.StoreInt64(&c.running, 0)
	close(c.waitCh)
}

// Done returns a channel that will send a signal when the Agent process is finished.
func (c *Commander) Done() <-chan struct{} {
	return c.doneCh
}

// Pid returns Agent process PID if it is started or 0 if it is not.
func (c *Commander) Pid() int {
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
	return atomic.LoadInt64(&c.running) != 0
}

// Stop the Agent process. Sends SIGTERM to the process and wait for up 10 seconds
// and if the process does not finish kills it forcedly by sending SIGKILL.
// Returns after the process is terminated.
func (c *Commander) Stop(ctx context.Context) error {
	c.startStopMutex.Lock()
	defer c.startStopMutex.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		// Not started, nothing to do.
		return nil
	}

	c.logger.Debugf(ctx, "Stopping agent process, PID=%v", c.cmd.Process.Pid)

	// Gracefully signal process to stop.
	if err := c.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	finished := make(chan struct{})

	// Setup a goroutine to wait a while for process to finish and send kill signal
	// to the process if it doesn't finish.
	var innerErr error
	go func() {
		// Wait 10 seconds.
		t := time.After(10 * time.Second)
		select {
		case <-ctx.Done():
			break
		case <-t:
			break
		case <-finished:
			// Process is successfully finished.
			c.logger.Debugf(ctx, "Agent process PID=%v successfully stopped.", c.cmd.Process.Pid)
			return
		}

		// Time is out. Kill the process.
		c.logger.Debugf(ctx,
			"Agent process PID=%d is not responding to SIGTERM. Sending SIGKILL to kill forcedly.",
			c.cmd.Process.Pid)
		if innerErr = c.cmd.Process.Signal(syscall.SIGKILL); innerErr != nil {
			return
		}
	}()

	// Wait for process to terminate
	<-c.waitCh

	atomic.StoreInt64(&c.running, 0)

	// Let goroutine know process is finished.
	close(finished)

	return innerErr
}
