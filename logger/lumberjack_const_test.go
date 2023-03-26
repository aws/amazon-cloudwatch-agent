// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logger

import (
	"log"
	"os"
	"testing"

	"github.com/influxdata/telegraf/logger"
	"github.com/stretchr/testify/assert"
)

func TestWriteLogToFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "")
	assert.NoError(t, err)
	defer func() { os.Remove(tmpfile.Name()) }()

	config := createBasicLogConfig(tmpfile.Name())
	logger.SetupLogging(config)
	log.Printf("I! TEST")
	log.Printf("D! TEST") // <- should be ignored

	f, err := os.ReadFile(tmpfile.Name())
	log.Printf("log: %v\n", string(f))
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z I! TEST\n"))
}

func TestDebugWriteLogToFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "")
	assert.NoError(t, err)
	defer func() { os.Remove(tmpfile.Name()) }()
	config := createBasicLogConfig(tmpfile.Name())
	config.Debug = true
	logger.SetupLogging(config)
	log.Printf("D! TEST")

	f, err := os.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z D! TEST\n"))
}

func TestErrorWriteLogToFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "")
	assert.NoError(t, err)
	defer func() { os.Remove(tmpfile.Name()) }()
	config := createBasicLogConfig(tmpfile.Name())
	config.Quiet = true
	logger.SetupLogging(config)
	log.Printf("E! TEST")
	log.Printf("I! TEST") // <- should be ignored

	f, err := os.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z E! TEST\n"))
}

func TestAddDefaultLogLevel(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "")
	assert.NoError(t, err)
	defer func() { os.Remove(tmpfile.Name()) }()
	config := createBasicLogConfig(tmpfile.Name())
	config.Debug = true
	logger.SetupLogging(config)
	log.Printf("TEST")

	f, err := os.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z I! TEST\n"))
}

func TestWriteToTruncatedFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "")
	assert.NoError(t, err)
	defer func() { os.Remove(tmpfile.Name()) }()
	config := createBasicLogConfig(tmpfile.Name())
	config.Debug = true
	logger.SetupLogging(config)
	log.Printf("TEST")

	f, err := os.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z I! TEST\n"))

	tmpf, err := os.OpenFile(tmpfile.Name(), os.O_RDWR|os.O_TRUNC, 0644)
	assert.NoError(t, err)
	assert.NoError(t, tmpf.Close())

	log.Printf("SHOULD BE FIRST")

	f, err = os.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z I! SHOULD BE FIRST\n"))
}

func createBasicLogConfig(filename string) logger.LogConfig {
	return logger.LogConfig{
		Logfile:             filename,
		LogTarget:           LogTargetLumberjack,
		RotationMaxArchives: -1,
		LogWithTimezone:     "UTC",
	}
}
