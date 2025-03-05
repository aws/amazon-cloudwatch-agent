// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package collectlist

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyKmsKeyIDRule(t *testing.T) {
	r := new(KmsKeyID)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"kms_key_id": "arn:aws:kms:us-west-2:1234567749:key/123452d-a2e4-4f1e-8bf2-08512341feb6"
	}`), &input)
	if e == nil {
		actualReturnKey, actualReturnVal := r.ApplyRule(input)
		assert.Equal(t, "kms_key_id", actualReturnKey)
		assert.Equal(t, "arn:aws:kms:us-west-2:1234567749:key/123452d-a2e4-4f1e-8bf2-08512341feb6", actualReturnVal)
	} else {
		panic(e)
	}
}

func TestInvalidKmsKeyIDRule(t *testing.T) {
	r := new(KmsKeyID)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"kms_key_id": 5
	}`), &input)
	if e == nil {
		actualReturnKey, actualReturnValue := r.ApplyRule(input)
		assert.Equal(t, "", actualReturnKey)
		assert.Equal(t, float64(5), actualReturnValue)
	} else {
		panic(e)
	}
}

func TestEmptyKmsKeyIDRule(t *testing.T) {
	r := new(KmsKeyID)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"kms_key_id": ""
	}`), &input)
	if e == nil {
		actualReturnKey, actualReturnValue := r.ApplyRule(input)
		assert.Equal(t, "", actualReturnKey)
		assert.Equal(t, "", actualReturnValue)
	} else {
		panic(e)
	}
}
