package agent

import (
	"context"
	"log"

	"github.com/open-telemetry/opamp-go/client/types"
)

var _ types.Logger = &Logger{}

type Logger struct {
	Logger *log.Logger
}

func (l *Logger) Debugf(_ context.Context, format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

func (l *Logger) Errorf(_ context.Context, format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}
