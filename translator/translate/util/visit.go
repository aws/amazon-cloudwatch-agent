// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"fmt"
	"strings"
)

const (
	Delimiter = "/"
)

var (
	ErrNoKeysFound     = errors.New("no keys found")
	ErrUnsupportedType = errors.New("unsupported type")
	ErrPathNotFound    = errors.New("path element not found")
	ErrTargetNotFound  = errors.New("target element not found")
)

type Visitor interface {
	Visit(value any) error
}

type funcVisitor func(value any) error

var _ Visitor = (funcVisitor)(nil)

func NewVisitor(fn func(value any) error) Visitor {
	return funcVisitor(fn)
}

func (v funcVisitor) Visit(value any) error {
	return v(value)
}

// SliceVisitor visits each element of a slice with the next visitor
type SliceVisitor struct {
	next Visitor
}

var _ Visitor = (*SliceVisitor)(nil)

func NewSliceVisitor(next Visitor) *SliceVisitor {
	return &SliceVisitor{next: next}
}

func (v *SliceVisitor) Visit(value any) error {
	s, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%w: %T", ErrUnsupportedType, value)
	}
	var errs []error
	for _, element := range s {
		if err := v.next.Visit(element); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// VisitPath represents a path through a nested map structure with each key separated by a Delimiter.
type VisitPath = string

// Path creates a VisitPath based on the keys.
func Path(keys ...string) VisitPath {
	return strings.Join(keys, Delimiter)
}

// Visit traverses the input following the path and calls the visitor with the result at the end of the path. If the
// path contains a slice, traverses each branch.
func Visit(input any, path VisitPath, visitor Visitor) error {
	if path == "" {
		return ErrNoKeysFound
	}
	keys := strings.Split(path, Delimiter)
	return visit(input, keys, visitor)
}

func visit(input any, keys []string, visitor Visitor) error {
	if len(keys) == 0 {
		if visitor == nil {
			return nil
		}
		return visitor.Visit(input)
	}
	switch current := input.(type) {
	case map[string]any:
		key := keys[0]
		value, ok := current[keys[0]]
		if !ok {
			baseErr := ErrPathNotFound
			if len(keys) == 1 {
				baseErr = ErrTargetNotFound
			}
			return fmt.Errorf("%w: %s", baseErr, key)
		}
		return visit(value, keys[1:], visitor)
	case []any:
		var errs []error
		for _, element := range current {
			if err := visit(element, keys, visitor); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
	default:
		return fmt.Errorf("%w: %T", ErrUnsupportedType, current)
	}
	return nil
}

// IsSet checks if the path exists in the input.
func IsSet(input any, path VisitPath) bool {
	err := Visit(input, path, nil)
	return err == nil
}
