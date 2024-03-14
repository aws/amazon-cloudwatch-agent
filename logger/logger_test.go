// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logger

import (
	"bufio"
	"strings"
	"testing"
	"time"

	"github.com/influxdata/wlog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

func TestConvertToAtomicLevel(t *testing.T) {
	type args struct {
		level wlog.Level
	}
	tests := []struct {
		name string
		args args
		want zap.AtomicLevel
	}{
		{
			name: "DEBUG",
			args: args{
				level: wlog.DEBUG,
			},
			want: zap.NewAtomicLevelAt(zapcore.DebugLevel),
		},
		{
			name: "INFO",
			args: args{
				level: wlog.INFO,
			},
			want: zap.NewAtomicLevelAt(zapcore.InfoLevel),
		},
		{
			name: "WARN",
			args: args{
				level: wlog.WARN,
			},
			want: zap.NewAtomicLevelAt(zapcore.WarnLevel),
		},
		{
			name: "ERROR",
			args: args{
				level: wlog.ERROR,
			},
			want: zap.NewAtomicLevelAt(zapcore.ErrorLevel),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ConvertToAtomicLevel(tt.args.level), "ConvertToAtomicLevel(%v)", tt.args.level)
		})
	}
}

func TestConvertToLetterLevel(t *testing.T) {
	type args struct {
		l zapcore.Level
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "DEBUG",
			args: args{
				l: zapcore.DebugLevel,
			},
			want: "D",
		},
		{
			name: "INFO",
			args: args{
				l: zapcore.InfoLevel,
			},
			want: "I",
		},
		{
			name: "WARN",
			args: args{
				l: zapcore.WarnLevel,
			},
			want: "W",
		},
		{
			name: "ERROR",
			args: args{
				l: zapcore.ErrorLevel,
			},
			want: "E",
		},
		{
			name: "FATAL",
			args: args{
				l: zapcore.FatalLevel,
			},
			want: "F",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ConvertToLetterLevel(tt.args.l), "ConvertToLetterLevel(%v)", tt.args.l)
		})
	}
}

func TestSetLevel(t *testing.T) {
	type args struct {
		level zap.AtomicLevel
	}
	type want struct {
		debug bool
		info  bool
		warn  bool
		error bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "DEBUG",
			args: args{
				level: zap.NewAtomicLevelAt(zapcore.DebugLevel),
			},
			want: want{
				debug: true,
				info:  true,
				warn:  true,
				error: true,
			},
		},
		{
			name: "INFO",
			args: args{
				level: zap.NewAtomicLevelAt(zapcore.InfoLevel),
			},
			want: want{
				debug: false,
				info:  true,
				warn:  true,
				error: true,
			},
		},
		{
			name: "WARN",
			args: args{
				level: zap.NewAtomicLevelAt(zapcore.WarnLevel),
			},
			want: want{
				debug: false,
				info:  false,
				warn:  true,
				error: true,
			},
		},
		{
			name: "ERROR",
			args: args{
				level: zap.NewAtomicLevelAt(zapcore.ErrorLevel),
			},
			want: want{
				debug: false,
				info:  false,
				warn:  false,
				error: true,
			},
		},
	}
	defer SetLevel(loggerLevel)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := buffer.NewPool().Get()
			logger, _ := zap.NewDevelopment(NewLoggerOptions(bufio.NewWriter(buf), zap.NewAtomicLevelAt(zapcore.InfoLevel))...)
			SetLevel(tt.args.level)
			logger.Debug("debug")
			logger.Info("info")
			logger.Warn("warn")
			logger.Error("error")
			assert.Equalf(t, loggerLevel, tt.args.level, "SetLevel(%v)", tt.args.level)
			assert.Equalf(t, strings.Contains("E!", buf.String()), tt.want.error, "found log line (%s) should find log line (%t)", buf.String(), tt.want.error)
			assert.Equalf(t, strings.Contains("W!", buf.String()), tt.want.error, "found log line (%s) should find log line (%t)", buf.String(), tt.want.warn)
			assert.Equalf(t, strings.Contains("I!", buf.String()), tt.want.error, "found log line (%s) should find log line (%t)", buf.String(), tt.want.info)
			assert.Equalf(t, strings.Contains("D!", buf.String()), tt.want.error, "found log line (%s) should find log line (%t)", buf.String(), tt.want.debug)
		})
	}
}

type stringer struct {
}

func (stringer stringer) String() string {
	return "any"
}

func TestTelegrafWrapperEncoder_EncodeEntry(t1 *testing.T) {
	type args struct {
		e zapcore.Entry
		f []zapcore.Field
	}
	currentTimestamp := time.Now()
	tests := []struct {
		name                   string
		telegrafWrapperEncoder TelegrafWrapperEncoder
		args                   args
	}{
		{
			name: "find message",
			args: args{
				e: zapcore.Entry{
					Level:      zapcore.InfoLevel,
					Time:       currentTimestamp,
					LoggerName: "any",
					Message:    "this is some message",
					Caller:     zapcore.EntryCaller{},
					Stack:      "any",
				},
				f: []zapcore.Field{
					{
						Key:       "any",
						Type:      zapcore.StringerType,
						String:    "any",
						Interface: stringer{},
					},
				},
			},
			telegrafWrapperEncoder: createTelegrafWrapperEncoder(),
		},
	}
	defer SetLevel(loggerLevel)
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			SetLevel(zap.NewAtomicLevelAt(tt.args.e.Level))
			got, err := tt.telegrafWrapperEncoder.EncodeEntry(tt.args.e, tt.args.f)
			assert.NoError(t1, err)
			assert.Contains(t1, got.String(), tt.args.e.Message, "EncodeEntry(%v, %v)", tt.args.e, tt.args.f)
		})
	}
}
