// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extract

import (
	"context"
	"strconv"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
)

const (
	portFlagLong     = "--port"
	portFlagShort    = "-P"
	portEnvVar       = "MYSQL_TCP_PORT"
	defaultMySQLPort = 3306
)

type portExtractor struct {
	subExtractors []detector.PortExtractor
}

// NewPortExtractor creates a port extractor that attempts to find the MySQL port
// from command line arguments (--port or -P flag) or environment variables (MYSQL_TCP_PORT).
// Falls back to the default MySQL port 3306.
func NewPortExtractor() detector.PortExtractor {
	return &portExtractor{
		subExtractors: []detector.PortExtractor{
			&cmdlinePortExtractor{},
			&envPortExtractor{},
		},
	}
}

func (e *portExtractor) Extract(ctx context.Context, process detector.Process) (int, error) {
	for _, sub := range e.subExtractors {
		port, err := sub.Extract(ctx, process)
		if err == nil {
			return port, nil
		}
	}
	return defaultMySQLPort, nil
}

// cmdlinePortExtractor extracts port from --port or -P flag
type cmdlinePortExtractor struct{}

func (e *cmdlinePortExtractor) Extract(ctx context.Context, process detector.Process) (int, error) {
	args, err := process.CmdlineSliceWithContext(ctx)
	if err != nil {
		return 0, err
	}

	for i, arg := range args {
		// --port=3307 or --port 3307
		if arg == portFlagLong && i+1 < len(args) {
			port, err := strconv.Atoi(args[i+1])
			if err == nil && util.IsValidPort(port) {
				return port, nil
			}
		}
		if strings.HasPrefix(arg, portFlagLong+"=") {
			port, err := strconv.Atoi(arg[len(portFlagLong)+1:])
			if err == nil && util.IsValidPort(port) {
				return port, nil
			}
		}
		// -P 3307 or -P3307
		if arg == portFlagShort && i+1 < len(args) {
			port, err := strconv.Atoi(args[i+1])
			if err == nil && util.IsValidPort(port) {
				return port, nil
			}
		}
		if strings.HasPrefix(arg, portFlagShort) && len(arg) > len(portFlagShort) {
			port, err := strconv.Atoi(arg[len(portFlagShort):])
			if err == nil && util.IsValidPort(port) {
				return port, nil
			}
		}
	}

	return 0, detector.ErrExtractPort
}

// envPortExtractor extracts port from MYSQL_TCP_PORT environment variable
type envPortExtractor struct{}

func (e *envPortExtractor) Extract(ctx context.Context, process detector.Process) (int, error) {
	env, err := process.EnvironWithContext(ctx)
	if err != nil {
		return 0, err
	}

	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 && parts[0] == portEnvVar {
			port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err == nil && util.IsValidPort(port) {
				return port, nil
			}
		}
	}

	return 0, detector.ErrExtractPort
}
