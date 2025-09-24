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
	jmxPortSystemProperty = "-Dcom.sun.management.jmxremote.port="
	jmxPortEnv            = "JMX_PORT="

	portNotFound = -1
)

// JmxPortExtractor is a singleton instance. It attempts to extract the JMX port from a given process by checking
// command line arguments and environment variables.
var JmxPortExtractor = newPortExtractor()

type portExtractor struct {
	subExtractors []detector.PortExtractor
}

func newPortExtractor() detector.PortExtractor {
	return &portExtractor{
		subExtractors: []detector.PortExtractor{new(cmdlinePortExtractor), new(envPortExtractor)},
	}
}

func (d *portExtractor) Extract(ctx context.Context, process detector.Process) (int, error) {
	var port int
	var err error
	for _, subDetector := range d.subExtractors {
		port, err = subDetector.Extract(ctx, process)
		if err == nil {
			break
		}
	}
	return port, err
}

type cmdlinePortExtractor struct {
}

var _ detector.PortExtractor = (*cmdlinePortExtractor)(nil)

func (d *cmdlinePortExtractor) Extract(ctx context.Context, process detector.Process) (int, error) {
	args, err := process.CmdlineSliceWithContext(ctx)
	if err != nil {
		return portNotFound, err
	}
	if len(args) <= 1 {
		return portNotFound, detector.ErrExtractPort
	}
	return extractPortFromSlice(args[1:], jmxPortSystemProperty)
}

type envPortExtractor struct {
}

var _ detector.PortExtractor = (*envPortExtractor)(nil)

func (d *envPortExtractor) Extract(ctx context.Context, process detector.Process) (int, error) {
	env, err := process.EnvironWithContext(ctx)
	if err != nil {
		return portNotFound, err
	}
	return extractPortFromSlice(env, jmxPortEnv)
}

func extractPortFromSlice(slice []string, prefix string) (int, error) {
	var portStr string
	for _, element := range slice {
		if strings.HasPrefix(element, prefix) {
			portStr = strings.TrimSpace(element[len(prefix):])
			break
		}
	}
	if portStr == "" {
		return portNotFound, detector.ErrExtractPort
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return portNotFound, detector.ErrInvalidPort
	}
	if !util.IsValidPort(port) {
		return portNotFound, detector.ErrInvalidPort
	}
	return port, nil
}
