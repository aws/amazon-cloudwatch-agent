// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package volume

type hostProvider struct {
}

func NewHostProvider() Provider {
	return &hostProvider{}
}

func (*hostProvider) DeviceToSerialMap() (map[string]string, error) {
	return getBlockDevices()
}
