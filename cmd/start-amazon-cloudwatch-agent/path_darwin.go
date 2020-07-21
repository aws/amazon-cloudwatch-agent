// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"io"
)

//add a dummy function so that the package can compile on mac os
func startAgent(writer io.WriteCloser) error {
	return nil
}
