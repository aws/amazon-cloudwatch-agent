// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package testutil

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/tool/stdin"
)

func SetUpTestInputStream() chan<- string {
	inputChan := make(chan string)
	stdin.Scanln = func(answer ...interface{}) (int, error) {
		inputString := <-inputChan
		fmt.Println(inputString)
		*(answer[0].(*string)) = inputString
		return len(inputString), nil
	}
	return inputChan
}

func SetPrometheusRemoteWriteTestingEnv(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "amazing_access_key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "super_secret_key")
	t.Setenv("AWS_REGION", "us-east-1")
}

func Type(inputChan chan<- string, inputString ...string) {
	go func() {
		for _, s := range inputString {
			inputChan <- s
		}
	}()
}

type LogSink struct {
	mu    sync.Mutex
	lines []string
}

var _ telegraf.Logger = (*LogSink)(nil)

func NewLogSink() *LogSink {
	return &LogSink{
		lines: make([]string, 0),
	}
}

func (l *LogSink) Errorf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, "E! "+fmt.Sprintf(format, args...))
}

func (l *LogSink) Error(args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, "E! "+fmt.Sprint(args...))
}

func (l *LogSink) Debugf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, "D! "+fmt.Sprintf(format, args...))
}

func (l *LogSink) Debug(args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, "D! "+fmt.Sprint(args...))
}

func (l *LogSink) Warnf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, "W! "+fmt.Sprintf(format, args...))
}

func (l *LogSink) Warn(args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, "W! "+fmt.Sprint(args...))
}

func (l *LogSink) Infof(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, "I! "+fmt.Sprintf(format, args...))
}

func (l *LogSink) Info(args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, "I! "+fmt.Sprint(args...))
}

func (l *LogSink) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	lines := make([]string, len(l.lines))
	copy(lines, l.lines)
	return lines
}

func (l *LogSink) String() string {
	return strings.Join(l.Lines(), "\n")
}

type NopLogger struct {
}

var _ telegraf.Logger = (*NopLogger)(nil)

func NewNopLogger() telegraf.Logger {
	return &NopLogger{}
}

func (n NopLogger) Errorf(string, ...interface{}) {
}

func (n NopLogger) Error(...interface{}) {
}

func (n NopLogger) Debugf(string, ...interface{}) {
}

func (n NopLogger) Debug(...interface{}) {
}

func (n NopLogger) Warnf(string, ...interface{}) {
}

func (n NopLogger) Warn(...interface{}) {
}

func (n NopLogger) Infof(string, ...interface{}) {
}

func (n NopLogger) Info(...interface{}) {
}
