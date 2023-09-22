// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tool

import (
	"bytes"
	"compress/gzip"
	"io"
)

func Compress(uncompressed []byte) ([]byte, error) {
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(uncompressed); err != nil {
		return nil, err
	}
	if err := gz.Flush(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	return compressed.Bytes(), nil
}

func Uncompress(compressed []byte) ([]byte, error) {
	compressedReader := bytes.NewReader(compressed)
	reader, err := gzip.NewReader(compressedReader)
	if err != nil {
		return nil, err
	}
	uncompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return uncompressed, nil
}
