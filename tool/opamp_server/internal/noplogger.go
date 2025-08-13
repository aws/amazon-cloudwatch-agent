package internal

import (
	"context"

	"github.com/open-telemetry/opamp-go/client/types"
)

var _ types.Logger = &NopLogger{}

type NopLogger struct{}

func (l *NopLogger) Debugf(ctx context.Context, format string, v ...interface{}) {}
func (l *NopLogger) Errorf(ctx context.Context, format string, v ...interface{}) {}
