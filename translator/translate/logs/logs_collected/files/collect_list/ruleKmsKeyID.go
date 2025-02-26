// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

const KmsKeyIDSectionKey = "kms_key_id"

type KmsKeyID struct {
}

func (l *KmsKeyID) ApplyRule(input interface{}) (string, interface{}) {
	_, val := translator.DefaultCase(KmsKeyIDSectionKey, "", input)
	if val == "" {
		return "", val
	}
	strVal, ok := val.(string)
	if !ok || strVal == "" {
		return "", val
	}
	key := "kms_key_id"
	val = util.ResolvePlaceholder(strVal, logs.GlobalLogConfig.MetadataInfo)
	return key, val
}

func init() {
	l := new(KmsKeyID)
	r := []Rule{l}
	RegisterRule(KmsKeyIDSectionKey, r)
}
