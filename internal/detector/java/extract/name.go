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

	// metaManifestFile is the path to the Manifest in the Java archive.
	metaManifestFile = "META-INF/MANIFEST.MF"
	// metaManifestApplicationName is a non-standard, but explicit field that should be used if present.
	metaManifestApplicationName = "Application-Name"
	// metaManifestImplementationTitle is a standard field that is conventionally used to name the application.
	metaManifestImplementationTitle = "Implementation-Title"
	// metaManifestStartClass is a Spring Boot specific field that should be used instead of Main-Class if present.
	metaManifestStartClass = "Start-Class"
	// metaManifestMainClass is the standard Java entry point. For frameworks like Spring Boot, this will point to the
	// framework launcher.
	metaManifestMainClass = "Main-Class"
)

var (
	// defaultManifestFieldPriority defines the priority order of the manifest fields.
	defaultManifestFieldPriority = []string{
		metaManifestApplicationName,
		metaManifestImplementationTitle,
		metaManifestStartClass,
		metaManifestMainClass,
	}
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
	name, err := e.extract(ctx, process)
	if err != nil {
		return "", err
	}
	// fallback on extracted name argument
	if e.skipByName.Contains(name) {
		return "", detector.ErrSkipProcess
	}
	for _, extractor := range e.subExtractors {
		var extractedName string
		extractedName, err = extractor.Extract(ctx, process, name)
		if err == nil && extractedName != "" {
			name = extractedName
			break
		}
	}
	return name, nil
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
	logger        *slog.Logger
	fieldPriority []string
	fieldLookup   collections.Set[string]
}

var _ argNameExtractor = (*archiveManifestNameExtractor)(nil)

func newArchiveManifestNameExtractor(logger *slog.Logger) argNameExtractor {
	return &archiveManifestNameExtractor{
		logger:        logger,
		fieldPriority: defaultManifestFieldPriority,
		fieldLookup:   collections.NewSet(defaultManifestFieldPriority...),
	}
}

// Extract opens the archive and reads the manifest file. Tries to extract the name from values of specific keys in the
// manifest. Prioritizes keys in the order defined in the extractor.
func (e *archiveManifestNameExtractor) Extract(ctx context.Context, process detector.Process, arg string) (string, error) {
	if !strings.HasSuffix(arg, extJAR) && !strings.HasSuffix(arg, extWAR) {
		return "", detector.ErrIncompatibleExtractor
	}
	fallback := strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg))
	path, err := absPath(ctx, process, arg)
	e.logger.Debug("Trying to extract name from Java Archive", "pid", process.PID(), "path", path)
	if err != nil {
		return fallback, nil
	}
	var name string
	if name, err = e.readManifest(path); err != nil {
		return fallback, nil
	}
	if name != "" {
		return name, nil
	}
	return fallback, nil
}

func (e *archiveManifestNameExtractor) readManifest(jarPath string) (string, error) {
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var manifestFile *zip.File
	for _, f := range r.File {
		if f.Name == metaManifestFile {
			manifestFile = f
			break
		}
	}
	if manifestFile == nil {
		return "", detector.ErrIncompatibleExtractor
	}
	rc, err := manifestFile.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	return e.parseManifest(rc), nil
}

func (e *archiveManifestNameExtractor) parseManifest(r io.Reader) string {
	manifest := make(map[string]string, len(e.fieldPriority))
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
			field := strings.TrimSpace(parts[0])
			if e.fieldLookup.Contains(field) {
				manifest[field] = strings.TrimSpace(parts[1])
				// exit early if the highest priority field is found
				if field == e.fieldPriority[0] && manifest[field] != "" {
					return manifest[field]
				}
			}
		}
	}
	for _, field := range e.fieldPriority {
		if name := manifest[field]; name != "" {
			return name
		}
	}
	return ""
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
