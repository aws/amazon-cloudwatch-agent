// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windowsevents

import "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"

type FileStateFolder struct {
}

// We are not exposing this field to customer
func (f *FileStateFolder) ApplyRule(_ interface{}) (string, interface{}) {
	return "file_state_folder", util.GetFileStateFolder()
}

func init() {
	RegisterRule("file_state_folder", new(FileStateFolder))
}
