// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
)

type FileStateFolder struct {
}

// FileStateFolder is internal value, not exposing to customer
func (f *FileStateFolder) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	return "file_state_folder", util.GetFileStateFolder()
}
func init() {
	f := new(FileStateFolder)
	RegisterRule("file_state_folder", f)
}
