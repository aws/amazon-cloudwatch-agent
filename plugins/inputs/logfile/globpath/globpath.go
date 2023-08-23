// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package globpath

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gobwas/glob"
)

var sepStr = fmt.Sprintf("%v", string(os.PathSeparator))

type GlobPath struct {
	path         string
	hasMeta      bool
	hasSuperMeta bool
	g            glob.Glob
	root         string
}

func Compile(path string) (*GlobPath, error) {
	out := GlobPath{
		hasMeta:      hasMeta(path),
		hasSuperMeta: hasSuperMeta(path),
		path:         path,
	}

	// if there are no glob meta characters in the path, don't bother compiling
	// a glob object or finding the root directory. (see short-circuit in Match)
	if !out.hasMeta && !out.hasSuperMeta {
		return &out, nil
	}

	// Escapes the `\` in windows since glob use it for escape sequence
	if runtime.GOOS == "windows" {
		path = escapeSeparator(path)
	}

	var err error
	if out.g, err = glob.Compile(path, os.PathSeparator); err != nil {
		return nil, err
	}
	// Get the root directory for this filepath
	out.root = findRootDir(path)
	return &out, nil
}

func (g *GlobPath) Match() map[string]os.FileInfo {
	if !g.hasMeta && !g.hasSuperMeta {
		out := make(map[string]os.FileInfo)
		info, err := os.Stat(g.path)
		if info != nil {
			out[g.path] = info
		} else {
			log.Printf("D! Stat file %v failed due to %v", g.path, err)
		}
		return out
	} else if !g.hasSuperMeta {
		out := make(map[string]os.FileInfo)
		files, _ := filepath.Glob(g.path)
		for _, file := range files {
			info, err := os.Stat(file)
			if info != nil {
				out[file] = info
			} else {
				log.Printf("D! Stat file %v failed due to %v", g.path, err)
			}
		}
		return out
	}
	return walkFilePath(g.root, g.g)
}

// walk the filepath from the given root and return a list of files that match
// the given glob.
func walkFilePath(root string, g glob.Glob) map[string]os.FileInfo {
	matchedFiles := make(map[string]os.FileInfo)
	walkfn := func(path string, info os.FileInfo, _ error) error {
		if info != nil && g.Match(path) {
			matchedFiles[path] = info
		}
		return nil
	}
	filepath.Walk(root, walkfn)
	return matchedFiles
}

// find the root dir of the given path (could include globs).
// ie:
//
//	/var/log/telegraf.conf -> /var/log
//	/home/** ->               /home
//	/home/*/** ->             /home
//	/lib/share/*/*/**.txt ->  /lib/share
func findRootDir(path string) string {
	pathItems := strings.Split(path, sepStr)
	out := sepStr
	for i, item := range pathItems {
		if i == len(pathItems)-1 {
			break
		}
		if item == "" {
			continue
		}
		if hasMeta(item) {
			break
		}
		out += item + sepStr
	}
	if out != sepStr {
		out = strings.TrimSuffix(out, sepStr)
		if runtime.GOOS == "windows" {
			out = strings.TrimPrefix(out, sepStr)
		}
	}
	return out
}

// escapeSeparator escapes the windows path separator '\' in glob pattern
// old "\\" - first '\' escapes the following path separator
// new "\\\\" - first '\' escapes second '\' which will be used as escape indicator in glob pattern
//
//	the third '\' escapes the fourth '\ which is the windows path separator
//
// return val - a string ready to be used in glob pattern
func escapeSeparator(path string) string {
	return strings.Replace(path, "\\", "\\\\", -1)
}

// hasMeta reports whether path contains any magic glob characters.
func hasMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// hasSuperMeta reports whether path contains any super magic glob characters (**), or glob characters
// that are not supported by filepath.Glob (!{})
func hasSuperMeta(path string) bool {
	return strings.Contains(path, "**") || strings.ContainsAny(path, "!{}")
}
