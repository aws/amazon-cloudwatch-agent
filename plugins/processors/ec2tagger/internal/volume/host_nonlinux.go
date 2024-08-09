// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !linux

package volume

import (
	"errors"
)

type hostProvider struct {
}

func newHostProvider() Provider {
	return &hostProvider{}
}

func (*hostProvider) DeviceToSerialMap() (map[string]string, error) {
	return nil, errors.New("local block device retrieval only supported on linux")
}
