// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

const (
	defaultLineLimit = 300
)

var (
	ErrLineLimitExceeded = errors.New("line limit exceeded")
)

// ScanProperties parses key-value pairs from a reader and passes them to the fn. Returns early if fn returns false,
// the line limit is reached, or if the scanner has an error. Uses the defaultLineLimit.
func ScanProperties(r io.Reader, separator rune, fn func(key, value string) bool) error {
	return scanPropertiesWithLimit(r, separator, defaultLineLimit, fn)
}

// scanPropertiesWithLimit parses key-value pairs from a reader and passes them to the fn. Returns early if fn returns
// false, the line limit is reached, or if the scanner has an error. A line limit of 0 means there is no limit.
func scanPropertiesWithLimit(r io.Reader, separator rune, lineLimit int, fn func(key, value string) bool) error {
	scanner := bufio.NewScanner(r)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineLimit > 0 && lineCount > lineLimit {
			return ErrLineLimitExceeded
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		index := strings.IndexRune(line, separator)
		if index == -1 {
			continue
		}
		key := strings.TrimSpace(line[:index])
		value := strings.TrimSpace(line[index+1:])
		if !fn(key, value) {
			return nil
		}
	}
	return scanner.Err()
}
