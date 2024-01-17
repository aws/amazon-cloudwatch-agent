// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logger

import (
	"io"
        "os"
	"time"

	"github.com/influxdata/wlog"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var (
	loggerLevel zap.AtomicLevel
)

type TelegrafWrapperEncoder struct {
	zapcore.Encoder
}

func NewLoggerOptions(writer io.Writer, level zap.AtomicLevel) []zap.Option {
	loggerLevel.SetLevel(level.Level())
	loggingOptions := getLoggingOptions(writer)

	return loggingOptions
}

func getLoggingOptions(writer io.Writer) []zap.Option {
	core := zapcore.NewCore(
		createTelegrafWrapperEncoder(),
		zapcore.AddSync(writer),
		loggerLevel,
	)
	option := zap.WrapCore(func(zapcore.Core) zapcore.Core {
		return core
	})
	return []zap.Option{option}
}

func createTelegrafWrapperEncoder() TelegrafWrapperEncoder {
	return TelegrafWrapperEncoder{
		zapcore.NewJSONEncoder(newProductionEncoderConfig()),
	}
}

func SetLevel(level zap.AtomicLevel) {
	loggerLevel.SetLevel(level.Level())
}

func (t TelegrafWrapperEncoder) EncodeEntry(e zapcore.Entry, f []zapcore.Field) (*buffer.Buffer, error) {
	entry, err := t.Encoder.EncodeEntry(e, f)
	if err != nil {
		return nil, err
	}
	buf := buffer.NewPool().Get()
	levelLetter := ConvertToLetterLevel(e.Level)
	buf.AppendString(levelLetter + "! ")
	buf.AppendString(entry.String())
	return buf, nil
}

func (t TelegrafWrapperEncoder) Clone() zapcore.Encoder {
	return TelegrafWrapperEncoder{
		zapcore.NewJSONEncoder(newProductionEncoderConfig()),
	}
}

func newProductionEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		NameKey:       "logger",
		CallerKey:     "caller",
		FunctionKey:   zapcore.OmitKey,
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}
}

func ConvertToAtomicLevel(level wlog.Level) zap.AtomicLevel {
	if level == wlog.DEBUG {
		return zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else if level == wlog.WARN {
		return zap.NewAtomicLevelAt(zapcore.WarnLevel)
	} else if level == wlog.ERROR {
		return zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	}
	return zap.NewAtomicLevelAt(zapcore.InfoLevel)
}

func ConvertToLetterLevel(l zapcore.Level) string {
	return string(l.CapitalString()[0])
}

func SampledLogger() *zap.Logger {
	stdout := zapcore.AddSync(os.Stdout)

	level := zap.NewAtomicLevelAt(zap.InfoLevel)
	productionCfg := newProductionEncoderConfig()
	productionCfg.TimeKey = "timestamp"
	productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	jsonEncoder := zapcore.NewJSONEncoder(productionCfg)

	jsonOutCore := zapcore.NewCore(jsonEncoder, stdout, level)

	// Create a logger that samples every Nth message after the first M messages every S seconds
	// where N = sc.Thereafter, M = sc.Initial, S = sc.Tick.
	samplingCore := zapcore.NewSamplerWithOptions(
		jsonOutCore,
		time.Hour, // interval
		5,           // log first 3 entries
		500,           // thereafter log zero entires within the interval
	)
	return zap.New(samplingCore)
}
func init() {
	loggerLevel = zap.NewAtomicLevelAt(zapcore.InfoLevel)
}

