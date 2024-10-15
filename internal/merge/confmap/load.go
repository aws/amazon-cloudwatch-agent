// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package confmap

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Loader interface {
	Load() (*Conf, error)
}

type FileLoader struct {
	path string
}

func NewFileLoader(path string) *FileLoader {
	return &FileLoader{path: path}
}

func (f *FileLoader) Load() (*Conf, error) {
	// Clean the path before using it.
	content, err := os.ReadFile(filepath.Clean(f.path))
	if err != nil {
		return nil, fmt.Errorf("unable to read the file %v: %w", f.path, err)
	}
	return NewByteLoader(f.path, content).Load()
}

type ByteLoader struct {
	id      string
	content []byte
}

func NewByteLoader(id string, content []byte) *ByteLoader {
	return &ByteLoader{id: id, content: content}
}

func (b *ByteLoader) Load() (*Conf, error) {
	var rawConf map[string]any
	if err := yaml.Unmarshal(b.content, &rawConf); err != nil {
		return nil, fmt.Errorf("unable to unmarshal contents: %v: %w", b.id, err)
	}
	return NewFromStringMap(rawConf), nil
}
