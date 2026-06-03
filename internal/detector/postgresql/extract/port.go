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
	portFlag            = "-p"
	portEnvVar          = "PGPORT"
	defaultPostgresPort = 5432
)

type portExtractor struct {
	subExtractors []detector.PortExtractor
}

// NewPortExtractor creates a port extractor that attempts to find the PostgreSQL port
// from command line arguments (-p flag) or environment variables (PGPORT).
// Falls back to the default PostgreSQL port 5432.
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
	return defaultPostgresPort, nil
}

// cmdlinePortExtractor extracts port from -p flag
type cmdlinePortExtractor struct{}

func (e *cmdlinePortExtractor) Extract(ctx context.Context, process detector.Process) (int, error) {
	args, err := process.CmdlineSliceWithContext(ctx)
	if err != nil {
		return 0, err
	}

	for i, arg := range args {
		if arg == portFlag && i+1 < len(args) {
			port, err := strconv.Atoi(args[i+1])
			if err == nil && util.IsValidPort(port) {
				return port, nil
			}
		}
		if strings.HasPrefix(arg, portFlag) && len(arg) > len(portFlag) {
			port, err := strconv.Atoi(arg[len(portFlag):])
			if err == nil && util.IsValidPort(port) {
				return port, nil
			}
		}
	}

	return 0, detector.ErrExtractPort
}

// envPortExtractor extracts port from PGPORT environment variable
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
