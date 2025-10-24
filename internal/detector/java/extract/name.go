// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extract

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

const (
	extJAR = ".jar"
	extWAR = ".war"

	// metaManifestFile is the path to the Manifest in the Java archive.
	metaManifestFile = "META-INF/MANIFEST.MF"
	// metaManifestSeparator is the separator of the key/value pairs in the file.
	metaManifestSeparator = ':'
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
	filter        detector.NameFilter
	subExtractors []argNameExtractor
}

func NewNameExtractor(logger *slog.Logger, filter detector.NameFilter) detector.NameExtractor {
	return &nameExtractor{
		logger: logger,
		filter: filter,
		subExtractors: []argNameExtractor{
			newArchiveManifestNameExtractor(logger),
		},
	}
}

// Extract extracts the name argument from the process command-line and runs it through the sub-extractors. Attempts
// to apply the filter before running the sub-extractors and again after.
func (e *nameExtractor) Extract(ctx context.Context, process detector.Process) (string, error) {
	name, err := e.extract(ctx, process)
	if err != nil {
		return "", err
	}
	// fallback on extracted name argument
	if err = e.applyFilter(name); err != nil {
		return "", err
	}
	for _, extractor := range e.subExtractors {
		var extractedName string
		extractedName, err = extractor.Extract(ctx, process, name)
		if err == nil && extractedName != "" {
			name = extractedName
			break
		}
	}
	return name, e.applyFilter(name)
}

func (e *nameExtractor) applyFilter(name string) error {
	if e.filter != nil {
		name = filepath.Base(name)
		if !e.filter.ShouldInclude(name) {
			return fmt.Errorf("%w due to filtered name: %s", detector.ErrSkipProcess, name)
		}
	}
	return nil
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
	path, err := util.AbsPath(ctx, process, arg)
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
	return parseManifest(rc, e.fieldPriority, e.fieldLookup), nil
}

func parseManifest(r io.Reader, fieldPriority []string, fieldLookup collections.Set[string]) string {
	manifest := make(map[string]string, len(fieldPriority))
	_ = util.ReadProperties(r, metaManifestSeparator, func(key, value string) bool {
		if !fieldLookup.Contains(key) || value == "" {
			return true
		}
		manifest[key] = value
		return key != fieldPriority[0]
	})
	for _, field := range fieldPriority {
		if name := manifest[field]; name != "" {
			return name
		}
	}
	return ""
}
