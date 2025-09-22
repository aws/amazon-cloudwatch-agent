// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !linux

package nvme

import "errors"

func GetRawData(_ string) ([]byte, error) {
	return nil, errors.New("NVMe metrics not supported on this platform")
}
