// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package attributestocontext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslatorID(t *testing.T) {
	tr := NewTranslator(nil)
	assert.Equal(t, "attributestocontext", tr.ID().String())
}

func TestTranslatorTranslate(t *testing.T) {
	actions := []ActionMapping{
		{Key: "log_group", FromResourceAttribute: "aws.cloudwatch.log_group.destination"},
		{Key: "log_stream", FromResourceAttribute: "aws.cloudwatch.log_stream.destination"},
	}
	tr := NewTranslator(actions)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestTranslatorTranslateEmpty(t *testing.T) {
	tr := NewTranslator(nil)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}
