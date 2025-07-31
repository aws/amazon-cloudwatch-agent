// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUtil_Interface(t *testing.T) {
	var _ DeviceInfoProvider = &Util{}
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "Amazon Elastic Block Store", ebsNvmeModelName)
	assert.Equal(t, "Amazon EC2 NVMe Instance Storage", instanceStoreNvmeModelName)
	assert.Equal(t, uint32(0xEC2C0D7E), uint32(InstanceStoreMagicNumber))
}
