// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package toyamlconfig

import (
	"bytes"
	"log"

	"gopkg.in/yaml.v3"
)

func ToYamlConfig(val interface{}) string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	err := enc.Encode(val)
	if err != nil {
		log.Panicf("Encode to a valid YAML config fails because of %v", err)
	}
	return buf.String()
}
