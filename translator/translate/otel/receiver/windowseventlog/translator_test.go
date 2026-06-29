// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windowseventlog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslator_ID(t *testing.T) {
	tr := NewTranslator("system_0", "System", false, nil)
	assert.Equal(t, "windowseventlog/system_0", tr.ID().String())
}

func TestTranslator_Translate_Structured(t *testing.T) {
	resource := map[string]string{
		"aws.log.source":     "windows_events",
		"aws.log.group.name": "/aws/windows/System",
	}
	tr := NewTranslator("system_0", "System", false, resource)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestTranslator_Translate_Raw(t *testing.T) {
	tr := NewTranslator("system_0", "System", true, nil)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}
