//go:build windows
// +build windows

package tail

import (
	"os"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/logfile/tail/winfile"
)

func OpenFile(name string) (file *os.File, err error) {
	return winfile.OpenFile(name, os.O_RDONLY, 0)
}
