package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

type flagDef struct {
	name        string
	value       *string
	defaultVal  string
	description string
}

func main() {
	log.Printf("Starting config-translator, this will map back to a call to amazon-cloudwatch-agent")

	flags := []flagDef{
		{"os", nil, "", "Please provide the os preference, valid value: windows/linux."},
		{"input", nil, "", "Please provide the path of input agent json config file"},
		{"input-dir", nil, "", "Please provide the path of input agent json config directory."},
		{"output", nil, "", "Please provide the path of the output CWAgent config file"},
		{"mode", nil, "ec2", "Please provide the mode, i.e. ec2, onPremise, onPrem, auto"},
		{"config", nil, "", "Please provide the common-config file"},
		{"multi-config", nil, "remove", "valid values: default, append, remove"},
	}

	for i := range flags {
		flags[i].value = flag.String(flags[i].name, flags[i].defaultVal, flags[i].description)
	}
	flag.Parse()

	args := []string{"-config-translator"}
	for _, f := range flags {
		if *f.value != "" {
			// prefix ct so we do not accidentally overlap with other agent flags
			args = append(args, fmt.Sprintf("-ct-%s", f.name), *f.value)
		}
	}

	log.Printf("Executing %s with arguments: %v", paths.AgentBinaryPath, args)

	cmd := exec.Command(paths.AgentBinaryPath, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Panicf("E! Translation process exited with non-zero status: %d, err: %v", exitErr.ExitCode(), exitErr)
		}
		log.Panicf("E! Translation process failed. Error: %v", err)
		os.Exit(1)
	}
}
