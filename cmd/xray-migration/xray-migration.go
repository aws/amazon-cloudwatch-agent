// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/tool/util"
	"github.com/aws/amazon-cloudwatch-agent/tool/xraydaemonmigration"
)

func main() {
	//downtime flag so user can configure their downtime before calling the migration tool.
	downTime := flag.Int("downTime", 5, "Set Daemon restart delay on CloudWatch failure. Note: trace data in this interval might be lost")
	flag.Parse()
	fmt.Printf("If the Cloudwatch agent doesn’t start within %v seconds, the X-ray daemon will restart. While the CloudWatch agent is starting, trace data generated during that time is lost.\n", *downTime)
	var userInput string
	fmt.Println("Enter 0 to cancel and exit, or enter a different number of seconds to wait for the CloudWatch agent to start (minimum of 2 seconds). Any trace data generated while waiting for the CloudWatch agent to start is lost.")
	fmt.Scanln(&userInput)
	var timeOut int
	var err error
	//assigning timeout duration
	if userInput == "" {
		//make sure downtime is at least 2 seconds
		if *downTime < 2 {
			timeOut = 2
		} else {
			timeOut = *downTime
		}
	} else {
		timeOut, err = strconv.Atoi(userInput)
		if err != nil {
			fmt.Println("Input given is not a number", err)
			os.Exit(1)
		}
		if timeOut == 0 {
			os.Exit(1)
		}
	}

	processes, _ := xraydaemonmigration.FindAllDaemons()
	//Can not migrate if Daemon is not running
	if len(processes) == 0 {
		fmt.Println("X-Ray Daemon is not running")
		return
	}
	//Before running wizard need to know if we need to use the user current configuration
	fetchOrAppend := configExists(defaultConfigLocation)
	err = RunWizard(fetchOrAppend)
	if err != nil {
		fmt.Println("There was a problem trying to run config wizard", err)
		os.Exit(1)
	}

	//Save Daemon Information before shutting down for restart Daemon function
	argList, _ := processes[0].CmdlineSlice()
	cwd, _ := processes[0].Cwd()
	isService := checkXrayStatus()
	err = TerminateXray(processes[0], checkXrayStatus)
	if err != nil {
		fmt.Println("There was a problem terminating X-Ray Daemon: ", err)
		os.Exit(1)
	}
	//Call Fetch or Append config depending on if user already has Daemon configuration
	if fetchOrAppend == Fetch {
		err = FetchConfig()
	} else {
		err = AppendConfig()
	}
	//need to restart Daemon if Fetch/Append does not work or CWA does not start within the timeout duration
	if err != nil || !IsCWAOn(time.Duration(timeOut)*time.Second, checkCWAStatus) {
		fmt.Println("There was a problem starting the Cloudwatch Agent. Restarting X-Ray Daemon")
		err := restartDaemon(argList[0], argList[1:], cwd, isService)
		if err != nil {
			fmt.Println("Could not restart X-Ray Daemon: ", err)
			return
		} else {
			fmt.Println("X-Ray Daemon has been restarted")
			return
		}
	} else {
		fmt.Println("Cloudwatch Agent has started and it is running traces!")
	}
}

// CmdInterface defines the methods we need from exec.Cmd.
type CmdInterface interface {
	Start() error
	Wait() error
	SetDir(string)
	SetSysProcAttr(*syscall.SysProcAttr)
	SetStdout(*os.File)
	SetStderr(*os.File)
}

type CmdWrapper struct {
	*exec.Cmd
}

func (cw *CmdWrapper) Start() error {
	return cw.Cmd.Start()
}

func (cw *CmdWrapper) Wait() error {
	return cw.Cmd.Wait()
}

func (cw *CmdWrapper) SetDir(dir string) {
	cw.Cmd.Dir = dir
}

func (cw *CmdWrapper) SetSysProcAttr(attr *syscall.SysProcAttr) {
	cw.Cmd.SysProcAttr = attr
}

func (cw *CmdWrapper) SetStdout(out *os.File) {
	cw.Cmd.Stdout = out
}

func (cw *CmdWrapper) SetStderr(err *os.File) {
	cw.Cmd.Stderr = err
}

var execCommand = func(name string, args ...string) CmdInterface {
	cmd := exec.Command(name, args...)
	return &CmdWrapper{
		Cmd: cmd,
	}
}

func RunWizard(fetchOrAppend startType) error {
	var err error
	var cmd *exec.Cmd
	if fetchOrAppend == Append {
		//need to save generated traces file in a different file
		cmd = exec.Command(pathToWizard, "-tracesOnly", "-configOutputPath", filepath.Join(pathToWizardDir, "config-traces.json"), "-nonInteractiveXrayMigration", "true")
	} else {
		cmd = exec.Command(pathToWizard, "-tracesOnly", "-nonInteractiveXrayMigration", "true")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return err
}

type startType int64

const (
	Fetch  startType = 0
	Append startType = 1
)

func configExists(configLocation string) startType {
	_, err := os.Stat(configLocation)
	//if file does not exist we just need to make traces file and fetch
	if os.IsNotExist(err) {
		return Fetch
	}
	//need to append current config with generated traces
	return Append
}

func TerminateXray(process xraydaemonmigration.Process, checkXrayStatus func() bool) error {
	var err error
	if checkXrayStatus() {
		err = StopXrayService()
	} else {
		err = process.Terminate()
	}
	return err
}

func IsCWAOn(timeout time.Duration, checkCWAStatus func() bool) bool {
	startTime := time.Now()
	//Check for duration of timeout
	for {
		if time.Since(startTime) > timeout {
			return false
		}
		if checkCWAStatus() {
			return true
		}
		time.Sleep(time.Second)
	}
}

func restartDaemon(daemonPath string, daemonArgs []string, cwd string, isService bool) error {
	var err error
	if isService {
		var cmd CmdInterface
		curOs := util.CurOS()
		if curOs == "windows" {
			cmd = execCommand("net", "start", "XRay")
		} else {
			cmd = execCommand("sudo", "systemctl", "start", "xray")
		}
		cmd.SetSysProcAttr(newSysProcAttr())
		cmd.SetStdout(os.Stdout)
		cmd.SetStderr(os.Stderr)
		err := cmd.Start()
		return err
	}

	cmd := execCommand(daemonPath, daemonArgs...)
	cmd.SetDir(cwd)
	cmd.SetSysProcAttr(newSysProcAttr())
	cmd.SetStdout(os.Stdout)
	err = cmd.Start()
	return err
}
