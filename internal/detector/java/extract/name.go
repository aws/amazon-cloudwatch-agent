// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extract

import (
	"archive/zip"
	"bufio"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

const (
	extJAR = ".jar"
	extWAR = ".war"

	metaManifestFile                = "META-INF/MANIFEST.MF"
	metaManifestStartClass          = "Start-Class"
	metaManifestImplementationTitle = "Implementation-Title"
	metaManifestMainClass           = "Main-Class"
)

type nameExtractor struct {
	logger        *slog.Logger
	skipByName    collections.Set[string]
	subExtractors []argNameExtractor
}

func NewNameExtractor(logger *slog.Logger, skipByName collections.Set[string]) detector.NameExtractor {
	return &nameExtractor{
		logger:     logger,
		skipByName: skipByName,
		subExtractors: []argNameExtractor{
			newArchiveManifestNameExtractor(logger),
		},
	}
}

func (e *nameExtractor) Extract(ctx context.Context, process detector.Process) (string, error) {
	arg, err := e.extract(ctx, process)
	if err != nil {
		return "", err
	}
	if e.skipByName.Contains(arg) {
		return "", detector.ErrSkipProcess
	}
	for _, extractor := range e.subExtractors {
		var name string
		name, err = extractor.Extract(ctx, process, arg)
		if err == nil && name != "" {
			return name, nil
		}
	}
	// fallback on extracted name argument
	return arg, nil
}

// extract finds the first argument that looks like a name.
//
//  1. Skips all flag arguments.
//     a. If the flag doesn't include an assignment and isn't a name flag (-jar, -m, etc.), skips the next argument
//     as well.
//  2. Skips all argument files. (TODO: Support argument files)
func (e *nameExtractor) extract(ctx context.Context, process detector.Process) (string, error) {
	args, err := process.CmdlineSliceWithContext(ctx)
	if err != nil {
		return "", err
	}
	var skipNextArg bool
	for _, arg := range args[1:] {
		if len(arg) == 0 {
			continue
		}
		if hasFlagPrefix(arg) {
			if isAssignmentFlag(arg) {
				skipNextArg = false
				continue
			}
			skipNextArg = !isNameFlag(arg)
			continue
		}
		if isArgumentFile(arg) {
			skipNextArg = false
			continue
		}
		if skipNextArg {
			skipNextArg = false
			continue
		}
		return arg, nil
	}
	return "", detector.ErrExtractName
}

// hasFlagPrefix identifies an argument that looks like a Java option flag.
func hasFlagPrefix(arg string) bool {
	return arg[0] == '-'
}

// isArgumentFile if the argument is a command-line argument file (@-file). The argument file allows command-line
// arguments to be defined in a space or newline delimited file.
// See https://docs.oracle.com/en/java/javase/17/docs/specs/man/java.html
func isArgumentFile(arg string) bool {
	return arg[0] == '@'
}

// isAssignmentFlag if the argument includes an assignment within them and is not expecting a subsequent value
// argument.
func isAssignmentFlag(arg string) bool {
	return strings.HasPrefix(arg, "-X") ||
		strings.HasPrefix(arg, "-javaagent:") ||
		strings.HasPrefix(arg, "-verbose:") ||
		strings.HasPrefix(arg, "-D") ||
		strings.ContainsRune(arg, '=')
}

// isNameFlag if the argument is a flag that is typically followed by the name argument.
func isNameFlag(arg string) bool {
	switch arg {
	case "-jar", "-m", "--module":
		return true
	default:
		return false
	}
}

type argNameExtractor interface {
	Extract(ctx context.Context, process detector.Process, arg string) (string, error)
}

type archiveManifestNameExtractor struct {
	logger *slog.Logger
}

var _ argNameExtractor = (*archiveManifestNameExtractor)(nil)

func newArchiveManifestNameExtractor(logger *slog.Logger) argNameExtractor {
	return &archiveManifestNameExtractor{logger: logger}
}

func (e *archiveManifestNameExtractor) Extract(ctx context.Context, process detector.Process, arg string) (string, error) {
	if !strings.HasSuffix(arg, extJAR) && !strings.HasSuffix(arg, extWAR) {
		return "", detector.ErrIncompatibleExtractor
	}
	fallback := strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg))
	path, err := absPath(ctx, process, arg)
	e.logger.Debug("Trying to extract name from Java Archive", "path", path)
	if err != nil {
		return fallback, nil
	}
	manifest, err := readManifest(path)
	if err != nil {
		return fallback, nil
	}
	name := nameFromManifest(manifest)
	if name != "" {
		return name, nil
	}
	return fallback, nil
}

func nameFromManifest(manifest map[string]string) string {
	order := []string{metaManifestStartClass, metaManifestImplementationTitle, metaManifestMainClass}
	for _, field := range order {
		if name, ok := manifest[field]; ok && name != "" {
			return name
		}
	}
	return ""
}

func readManifest(jarPath string) (map[string]string, error) {
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == metaManifestFile {
			var rc io.ReadCloser
			rc, err = f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			manifest := make(map[string]string)
			scanner := bufio.NewScanner(rc)
			for scanner.Scan() {
				line := scanner.Text()
				if kv := strings.SplitN(line, ":", 2); len(kv) == 2 {
					key := strings.TrimSpace(kv[0])
					manifest[key] = strings.TrimSpace(kv[1])
				}
			}
			return manifest, scanner.Err()
		}
	}
	return nil, nil
}

func absPath(ctx context.Context, process detector.Process, path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	cwd, err := process.CwdWithContext(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, path), nil
}
