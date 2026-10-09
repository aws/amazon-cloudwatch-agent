// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const KmsKeyIdSectionKey = "kms_key_id"

type KmsKeyId struct {
}

func (k *KmsKeyId) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(KmsKeyIdSectionKey, "", input)
	key, _ := returnVal.(string)
	key = strings.TrimSpace(key)
	if key == "" {
		return "", ""
	}
	return KmsKeyIdSectionKey, key
}

func init() {
	RegisterRule(KmsKeyIdSectionKey, []Rule{new(KmsKeyId)})
}
