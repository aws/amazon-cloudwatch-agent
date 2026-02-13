// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	assert.NoError(t, (&Config{}).Validate())
}
