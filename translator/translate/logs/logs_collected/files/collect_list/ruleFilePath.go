// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"strconv"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type FilePath struct {
}

func (f *FilePath) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	//Should be mandatory case
	if translator.IsValid(input, "file_path", GetCurPath()+"file_path"+strconv.Itoa(Index)) {
		returnKey, returnVal = translator.DefaultCase("file_path", "", input)
	} else {
		returnKey = ""
		returnVal = ""
	}
	return
}

func init() {
	fp := new(FilePath)
	r := []Rule{fp}
	RegisterRule("file_path", r)
}
