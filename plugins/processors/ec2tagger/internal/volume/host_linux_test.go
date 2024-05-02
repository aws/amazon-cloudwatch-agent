// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package volume

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testDirEntries = []os.DirEntry{
		&mockDirEntry{name: "xvdc"},
		&mockDirEntry{name: "loop1"},
		&mockDirEntry{name: "xvdc1"},
		&mockDirEntry{name: "xvdf"},
		&mockDirEntry{name: "loop2"},
		&mockDirEntry{name: "xvdh"},
	}
	testSerialMap = map[string]string{
		serialFilePath("xvdc"):  "vol-0303a1cc896c42d28",
		serialFilePath("xvdf"):  "vol0c241693efb58734a",
		serialFilePath("xvdh"):  "otherserial",
		serialFilePath("loop1"): "skip",
	}
)

type mockDirEntry struct {
	os.DirEntry
	name string
}

func (m *mockDirEntry) Name() string {
	return m.name
}

type mockFileSystem struct {
	serialMap map[string]string
	errDir    error
}

func (m *mockFileSystem) ReadDir(string) ([]os.DirEntry, error) {
	if m.errDir != nil {
		return nil, m.errDir
	}
	return testDirEntries, nil
}

func (m *mockFileSystem) ReadFile(path string) ([]byte, error) {
	return []byte(m.serialMap[path]), nil
}

func TestHostProvider(t *testing.T) {
	testErr := errors.New("test")
	m := &mockFileSystem{
		errDir: testErr,
	}
	p := newHostProvider().(*hostProvider)
	p.osReadDir = m.ReadDir
	p.osReadFile = m.ReadFile
	got, err := p.DeviceToSerialMap()
	assert.Error(t, err)
	assert.Nil(t, got)
	m.errDir = nil
	got, err = p.DeviceToSerialMap()
	assert.Error(t, err)
	assert.Nil(t, got)
	m.serialMap = testSerialMap
	got, err = p.DeviceToSerialMap()
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"xvdc": "vol-0303a1cc896c42d28",
		"xvdf": "vol-0c241693efb58734a",
		"xvdh": "otherserial",
	}, got)
}
