// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"log"
)

const (
	noLogger = iota
)

type loggeriface interface {
	Log(...interface{})
}

func newLogger(loglevel int) loggeriface {
	switch loglevel {
	case 1:
		return stdoutLogger{}
	}

	return noopLogger{}
}

type noopLogger struct{}

func (l noopLogger) Log(o ...interface{}) {}

type stdoutLogger struct{}

func (l stdoutLogger) Log(list ...interface{}) {
	log.Println(list...)
}
