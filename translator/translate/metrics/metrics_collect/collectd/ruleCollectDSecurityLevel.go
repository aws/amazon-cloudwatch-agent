// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type SecurityLevel struct {
}

const SectionKey_SecurityLevel = "collectd_security_level"

func (obj *SecurityLevel) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_SecurityLevel, "encrypt", input)
	return
}

func init() {
	obj := new(SecurityLevel)
	RegisterRule(SectionKey_SecurityLevel, obj)
}
