// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package fdlimit

import (
	"github.com/stretchr/testify/assert"
	"runtime"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

func TestFileDescriptorsLimit(t *testing.T) {
	currentOpenFileLimit, err := CurrentOpenFileLimit()
	assert.NoError(t, err)

	if runtime.GOOS == config.OS_TYPE_WINDOWS {
		assert.Equal(t, 16384, currentOpenFileLimit)
	}
}
