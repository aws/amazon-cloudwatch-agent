// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logger

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/influxdata/telegraf/logger"
	"github.com/influxdata/wlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/natefinch/lumberjack.v2"
)

func TestWriteLogToFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "")
	assert.NoError(t, err)
	defer func() { os.Remove(tmpfile.Name()) }()

	config := createBasicLogConfig(tmpfile.Name())
	logger.SetupLogging(config)
	log.Printf("I! TEST")
	log.Printf("D! TEST") // <- should be ignored

	f, err := ioutil.ReadFile(tmpfile.Name())
	log.Printf("log: %v\n", string(f))
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z I! TEST\n"))
}

func TestDebugWriteLogToFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "")
	assert.NoError(t, err)
	defer func() { os.Remove(tmpfile.Name()) }()
	config := createBasicLogConfig(tmpfile.Name())
	config.Debug = true
	logger.SetupLogging(config)
	log.Printf("D! TEST")

	f, err := ioutil.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z D! TEST\n"))
}

func TestErrorWriteLogToFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "")
	assert.NoError(t, err)
	defer func() { os.Remove(tmpfile.Name()) }()
	config := createBasicLogConfig(tmpfile.Name())
	config.Quiet = true
	logger.SetupLogging(config)
	log.Printf("E! TEST")
	log.Printf("I! TEST") // <- should be ignored

	f, err := ioutil.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z E! TEST\n"))
}

func TestAddDefaultLogLevel(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "")
	assert.NoError(t, err)
	defer func() { os.Remove(tmpfile.Name()) }()
	config := createBasicLogConfig(tmpfile.Name())
	config.Debug = true
	logger.SetupLogging(config)
	log.Printf("TEST")

	f, err := ioutil.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z I! TEST\n"))
}

func TestWriteToTruncatedFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "")
	assert.NoError(t, err)
	defer func() { os.Remove(tmpfile.Name()) }()
	config := createBasicLogConfig(tmpfile.Name())
	config.Debug = true
	logger.SetupLogging(config)
	log.Printf("TEST")

	f, err := ioutil.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z I! TEST\n"))

	tmpf, err := os.OpenFile(tmpfile.Name(), os.O_RDWR|os.O_TRUNC, 0644)
	assert.NoError(t, err)
	assert.NoError(t, tmpf.Close())

	log.Printf("SHOULD BE FIRST")

	f, err = ioutil.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, f[19:], []byte("Z I! SHOULD BE FIRST\n"))
}

func TestWriteToFileInRotation(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "LogRotation")
	require.NoError(t, err)
	config := createBasicLogConfig(filepath.Join(tempDir, "test.log"))
	config.LogTarget = logger.LogTargetFile
	wlog.SetLevel(wlog.INFO)
	maxsize := 1
	lumberjackLogger := &lumberjack.Logger{
		Filename:   config.Logfile,
		MaxSize:    maxsize,
		MaxBackups: 5,
		MaxAge:     1,
		Compress:   false,
	}
	log.SetOutput(logger.NewTelegrafWriter(lumberjackLogger))
	var logWriter interface{} = lumberjackLogger
	// Close the writer here, otherwise the temp folder cannot be deleted because the current log file is in use.
	closer, isCloser := logWriter.(io.Closer)
	assert.True(t, isCloser)
	defer func() { closer.Close(); os.RemoveAll(tempDir) }()

	s := "I! TEST 2 " + strings.Repeat("a", maxsize*1024*1024-100)
	log.Printf(s) // Writes 1M bytes, will rotate

	files, _ := ioutil.ReadDir(tempDir)
	assert.Equal(t, 1, len(files))

	// make sure the length is less than maxsize M, otherwise logger will discard the line.
	log.Printf(s) // Writes 1M bytes, will rotate

	files, _ = ioutil.ReadDir(tempDir)
	assert.Equal(t, 2, len(files))

	log.Printf(s) // Writes 1M bytes, will rotate
	log.Printf(s) // Writes 1M bytes, will rotate

	files, _ = ioutil.ReadDir(tempDir)
	for _, file := range files {
		fmt.Printf("%v/%v, size:%v\n", tempDir, file.Name(), file.Size())
	}

	assert.Equal(t, 4, len(files))

}

func createBasicLogConfig(filename string) logger.LogConfig {
	return logger.LogConfig{
		Logfile:             filename,
		LogTarget:           logger.LogTargetFile,
		RotationMaxArchives: -1,
	}
}
