// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"fmt"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail/winfile"
)

func createTempFile(dir, prefix string) (*os.File, error) {
	file, err := os.CreateTemp(dir, prefix)
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("Failed to close created temp file %v: %w", file.Name(), err)
	}

	file, err = winfile.OpenFile(file.Name(), os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("Failed to open temp file for writing %v: %w", file.Name(), err)
	}
	return file, err
}
