// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extract

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
)

// https://tomcat.apache.org/tomcat-8.5-doc/introduction.html#CATALINA_HOME_and_CATALINA_BASE
//
// CATALINA_BASE: Represents the root of a runtime configuration of a specific Tomcat instance. If you want to have
// multiple Tomcat instances on one machine, use the CATALINA_BASE property.
//
// CATALINA_HOME: Represents the root of your Tomcat installation, for example /home/tomcat/apache-tomcat-9.0.10 or
// C:\Program Files\apache-tomcat-9.0.10.
//
// By default, CATALINA_HOME and CATALINA_BASE point to the same directory. Set CATALINA_BASE manually when you require
// running multiple Tomcat instances on one machine.
const (
	systemPropertyCatalinaBase = "-Dcatalina.base"
	systemPropertyCatalinaHome = "-Dcatalina.home"
	envCatalinaBase            = "CATALINA_BASE"
	envCatalinaHome            = "CATALINA_HOME"
)

// tomcatDirectories holds Catalina directory paths with base taking priority over home
type tomcatDirectories struct {
	base string // preferred
	home string // fallback
}

func (d *tomcatDirectories) trySetBase(base string) {
	if d.base == "" {
		d.base = base
	}
}

func (d *tomcatDirectories) trySetHome(home string) {
	if d.home == "" {
		d.home = home
	}
}

type tomcatDirExtractor = detector.Extractor[*tomcatDirectories]

type nameExtractor struct {
	logger        *slog.Logger
	subExtractors []tomcatDirExtractor
}

// NewNameExtractor creates a new extractor that tries to determine a Tomcat name. It checks the process command-line
// arguments first and falls back on the process environment variables.
func NewNameExtractor(logger *slog.Logger) detector.NameExtractor {
	return &nameExtractor{
		logger: logger,
		subExtractors: []tomcatDirExtractor{
			new(cmdlineTomcatDirExtractor),
			new(envTomcatDirExtractor),
		},
	}
}

// Extract returns CATALINA_BASE if available, otherwise CATALINA_HOME. If neither are available, then returns
// an error.
func (e *nameExtractor) Extract(ctx context.Context, process detector.Process) (string, error) {
	tomcatDirs := tomcatDirectories{}
	for _, extractor := range e.subExtractors {
		extractedDirs, err := extractor.Extract(ctx, process)
		if err == nil && extractedDirs != nil {
			tomcatDirs.trySetBase(extractedDirs.base)
			if tomcatDirs.base != "" {
				break
			}
			tomcatDirs.trySetHome(extractedDirs.home)
		}
	}
	if tomcatDirs.base != "" {
		return tomcatDirs.base, nil
	}
	if tomcatDirs.home != "" {
		return tomcatDirs.home, nil
	}
	return "", fmt.Errorf("%w: missing Tomcat properties", detector.ErrIncompatibleExtractor)
}

type cmdlineTomcatDirExtractor struct {
}

var _ tomcatDirExtractor = (*cmdlineTomcatDirExtractor)(nil)

func (d *cmdlineTomcatDirExtractor) Extract(ctx context.Context, process detector.Process) (*tomcatDirectories, error) {
	args, err := process.CmdlineSliceWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return extractTomcatDirectories(args, systemPropertyCatalinaBase, systemPropertyCatalinaHome), nil
}

type envTomcatDirExtractor struct {
}

var _ tomcatDirExtractor = (*envTomcatDirExtractor)(nil)

func (d *envTomcatDirExtractor) Extract(ctx context.Context, process detector.Process) (*tomcatDirectories, error) {
	env, err := process.EnvironWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return extractTomcatDirectories(env, envCatalinaBase, envCatalinaHome), nil
}

// extractTomcatDirectories parses a set of entries to find Tomcat directory paths.
func extractTomcatDirectories(entries []string, baseKey, homeKey string) *tomcatDirectories {
	dirs := &tomcatDirectories{}
	for _, entry := range entries {
		parts := strings.Split(entry, "=")
		if len(parts) == 2 {
			switch parts[0] {
			case baseKey:
				dirs.trySetBase(util.TrimQuotes(parts[1]))
			case homeKey:
				dirs.trySetHome(util.TrimQuotes(parts[1]))
			}
		}
		// if the tomcat base is found, exit early since it takes priority
		if dirs.base != "" {
			break
		}
	}
	return dirs
}
