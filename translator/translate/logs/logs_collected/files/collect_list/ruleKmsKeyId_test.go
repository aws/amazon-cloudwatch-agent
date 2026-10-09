// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

func TestKmsKeyIdRule(t *testing.T) {
	translator.ResetMessages()
	f := new(FileConfig)
	var input interface{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"collect_list": [{
			"file_path": "/var/log/app.log",
			"log_group_name": "app",
			"kms_key_id": " alias/app-logs "
		}]
	}`), &input))

	_, val := f.ApplyRule(input)
	configs := val.([]interface{})
	require.Len(t, configs, 1)
	assert.Equal(t, "alias/app-logs", configs[0].(map[string]interface{})["kms_key_id"])
	assert.Empty(t, translator.ErrorMessages)
}

func TestConflictingKmsKeyId(t *testing.T) {
	translator.ResetMessages()
	f := new(FileConfig)
	var input interface{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"collect_list": [
			{"file_path": "/a", "log_group_name": "app", "kms_key_id": "alias/one"},
			{"file_path": "/b", "log_group_name": "app", "kms_key_id": "alias/two"}
		]
	}`), &input))

	f.ApplyRule(input)
	require.NotEmpty(t, translator.ErrorMessages)
	assert.Contains(t, translator.ErrorMessages[len(translator.ErrorMessages)-1], "Different kms_key_id values can't be set for the same log group: app")
}
