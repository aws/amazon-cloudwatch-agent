// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package detector

import (
	"context"
	"errors"
)

var (
	ErrIncompatibleExtractor = errors.New("incompatible extractor")
	ErrExtractName           = errors.New("unable to extract name")
	ErrExtractPort           = errors.New("unable to extract port")
	ErrInvalidPort           = errors.New("invalid port")
)

type Extractor[T any] interface {
	Extract(ctx context.Context, process Process) (T, error)
}

type PortExtractor = Extractor[int]

type NameExtractor = Extractor[string]
