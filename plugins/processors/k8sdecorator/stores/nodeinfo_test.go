// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stores

import (
	"bufio"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/mapWithExpiry"
)

func TestNodeInfo_extractEbsId(t *testing.T) {
	nodeInfo := &nodeInfo{ebsIds: mapWithExpiry.NewMapWithExpiry(60 * time.Second)}
	file, _ := os.Open("./sampleMountFile/mounts")
	defer file.Close()
	reader := bufio.NewReader(file)

	nodeInfo.extractEbsId(reader)
	assert.Equal(t, 1, nodeInfo.ebsIds.Size())
	volId, _ := nodeInfo.ebsIds.Get("/dev/nvme1n1")
	assert.Equal(t, "aws://us-west-2b/vol-0d9f0816149eb2050", volId.(string))
}
