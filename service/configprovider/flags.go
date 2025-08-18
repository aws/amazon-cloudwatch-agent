// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"flag"
	"fmt"
)

const (
	OtelConfigFlagName = "otelconfig"
)

type OtelConfigFlags []string

var _ flag.Value = (*OtelConfigFlags)(nil)

func (o *OtelConfigFlags) String() string {
	return fmt.Sprint(*o)
}

func (o *OtelConfigFlags) Set(value string) error {
	*o = append(*o, value)
	return nil
}
