// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package confmap

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileLoader(t *testing.T) {
	loader := NewFileLoader(filepath.Join("not", "a", "file"))
	got, err := loader.Load()
	assert.Error(t, err)
	assert.Nil(t, got)
	loader = NewFileLoader(filepath.Join("testdata", "base.yaml"))
	got, err = loader.Load()
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

func TestByteLoader(t *testing.T) {
	testCase := `receivers:
  nop/1:
`
	loader := NewByteLoader("invalid-yaml", []byte("string"))
	got, err := loader.Load()
	assert.Error(t, err)
	assert.Nil(t, got)
	loader = NewByteLoader("valid-yaml", []byte(testCase))
	got, err = loader.Load()
	assert.NoError(t, err)
	assert.NotNil(t, got)
}
