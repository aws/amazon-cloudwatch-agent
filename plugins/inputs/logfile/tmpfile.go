//go:build !windows
// +build !windows

package logfile

import (
	"os"
)

func createTempFile(dir, prefix string) (*os.File, error) {
	return os.CreateTemp(dir, prefix)
}
