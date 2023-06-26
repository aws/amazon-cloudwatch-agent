// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type TypesDB struct {
}

const SectionKey_TypesDB = "collectd_typesdb"

func (obj *TypesDB) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_TypesDB, []interface{}{"/usr/share/collectd/types.db"}, input)
	return
}

func init() {
	obj := new(TypesDB)
	RegisterRule(SectionKey_TypesDB, obj)
}
