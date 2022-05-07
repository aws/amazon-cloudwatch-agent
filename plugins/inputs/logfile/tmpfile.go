//go:build !windows
// +build !windows

package logfile

import (
	"io/ioutil"
	"os"
)

func createTempFile(dir, prefix string) (*os.File, error) {
	return ioutil.TempFile(dir, prefix)
}
