// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestPagingFile_ToMap(t *testing.T) {
	expectedKey := "Paging File"
	expectedValue := map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"% Usage"}}
	ctx := &runtime.Context{}
	conf := new(PagingFile)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
