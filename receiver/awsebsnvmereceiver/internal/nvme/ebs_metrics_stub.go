// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !linux

package nvme

import "errors"

func GetMetrics(devicePath string) (EBSMetrics, error) {
	return EBSMetrics{}, errors.New("ebs metrics stub: ebs metrics not supported")
}
