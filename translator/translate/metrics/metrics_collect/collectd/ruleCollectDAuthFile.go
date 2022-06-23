// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type AuthFile struct {
}

const SectionKey_AuthFile = "collectd_auth_file"

func (obj *AuthFile) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_AuthFile, "/etc/collectd/auth_file", input)
	return
}

func init() {
	obj := new(AuthFile)
	RegisterRule(SectionKey_AuthFile, obj)
}
