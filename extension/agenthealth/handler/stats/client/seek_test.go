// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errorSeeker struct {
	io.Reader
}

func (e *errorSeeker) Seek(int64, int) (int64, error) {
	return 0, errors.New("seek error")
}

type readSeeker struct {
	r *bytes.Reader
}

func (s *readSeeker) Read(p []byte) (int, error) {
	return s.r.Read(p)
}

func (s *readSeeker) Seek(offset int64, whence int) (int64, error) {
	return s.r.Seek(offset, whence)
}

const testDataStr = "test data"

var testData = []byte(testDataStr)

func TestReaderSeekerCloser_Read(t *testing.T) {
	rsc := readSeekCloser(bytes.NewReader(testData))
	buf := make([]byte, 9)
	n, err := rsc.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 9, n)
	assert.Equal(t, testDataStr, string(buf))
}

func TestReaderSeekerCloser_Seek(t *testing.T) {
	rsc := readSeekCloser(bytes.NewReader(testData))

	offset, err := rsc.Seek(5, io.SeekStart)
	require.NoError(t, err)
	assert.EqualValues(t, 5, offset)

	buf := make([]byte, 4)
	n, err := rsc.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, "data", string(buf))
}

func TestReaderSeekerCloser_Close(t *testing.T) {
	t.Run("WithCloser", func(t *testing.T) {
		rsc := readSeekCloser(io.NopCloser(bytes.NewReader(testData)))
		require.NoError(t, rsc.Close())
	})

	t.Run("WithoutCloser", func(t *testing.T) {
		rsc := readSeekCloser(bytes.NewReader(testData))
		require.NoError(t, rsc.Close())
	})
}

func TestSeekerLen(t *testing.T) {
	t.Run("WithLen", func(t *testing.T) {
		// Uses Len() method when available
		buf := bytes.NewBuffer(testData)
		rsc := readSeekCloser(buf)
		length, err := seekerLen(rsc)
		require.NoError(t, err)
		assert.EqualValues(t, len(testData), length)
	})

	t.Run("WithSeek", func(t *testing.T) {
		// Falls back to Seek when Len() not available
		rsc := readSeekCloser(&readSeeker{r: bytes.NewReader(testData)})
		length, err := seekerLen(rsc)
		require.NoError(t, err)
		assert.EqualValues(t, len(testData), length)
	})

	t.Run("WithSeek/Error", func(t *testing.T) {
		es := &errorSeeker{Reader: bytes.NewReader(testData)}
		length, err := seekerLen(es)
		require.Error(t, err)
		assert.EqualValues(t, 0, length)
	})

	t.Run("WithoutLenOrSeek", func(t *testing.T) {
		// Returns -1 when neither Len() nor Seek() available
		rsc := readSeekCloser(&io.LimitedReader{R: bytes.NewReader(testData), N: int64(len(testData))})
		length, err := seekerLen(rsc)
		require.NoError(t, err)
		assert.EqualValues(t, -1, length)
	})

	t.Run("WithOffset", func(t *testing.T) {
		// Computes remaining length from current position
		seeker := bytes.NewReader(testData)
		_, err := seeker.Seek(5, io.SeekStart)
		assert.NoError(t, err)
		length, err := seekerLen(seeker)
		require.NoError(t, err)
		assert.EqualValues(t, 4, length)
	})
}
