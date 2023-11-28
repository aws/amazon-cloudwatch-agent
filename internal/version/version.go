// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package version

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	filename       = "CWAGENT_VERSION"
	unknownVersion = "Unknown"
)

var (
	version     = readVersionFile()
	fullVersion = buildFullVersion(version)
)

func Number() string {
	return version
}

func Full() string {
	return fullVersion
}

func FilePath() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(ex), filename), nil
}

func buildFullVersion(version string) string {
	return fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		version,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
}

func readVersionFile() string {
	versionFilePath, err := FilePath()
	if err != nil {
		return unknownVersion
	}
	content, err := os.ReadFile(versionFilePath)
	if err != nil {
		return unknownVersion
	}
	return strings.Trim(string(content), " \n\r\t")
}
