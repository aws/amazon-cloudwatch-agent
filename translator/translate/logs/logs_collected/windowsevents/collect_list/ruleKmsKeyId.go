// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const KmsKeyIdSectionKey = "kms_key_id"

type KmsKeyId struct {
}

func (k *KmsKeyId) ApplyRule(input interface{}) (string, interface{}) {
	_, returnVal := translator.DefaultCase(KmsKeyIdSectionKey, "", input)
	key, _ := returnVal.(string)
	key = strings.TrimSpace(key)
	if key == "" {
		return "", ""
	}
	return KmsKeyIdSectionKey, key
}

func init() {
	RegisterRule(KmsKeyIdSectionKey, new(KmsKeyId))
}
