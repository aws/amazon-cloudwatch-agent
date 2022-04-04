// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"runtime"
	"strings"
)

var supportedOs = [...]string{OS_TYPE_LINUX, OS_TYPE_WINDOWS, OS_TYPE_DARWIN}

const (
	OS_TYPE_LINUX   = "linux"
	OS_TYPE_WINDOWS = "windows"
	OS_TYPE_DARWIN  = "darwin"
)

func ToValidOs(os string) string {
	if os == "" {
		// Give it a last try, using current osType type
		os = runtime.GOOS
	}

	formattedOs := strings.ToLower(os)
	for _, val := range supportedOs {
		if formattedOs == val {
			return formattedOs
		}
	}

	panic(fmt.Sprintf("%v is not a supported osType type", os))
}
