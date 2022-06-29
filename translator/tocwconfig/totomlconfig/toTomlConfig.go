// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package totomlconfig

import (
	"bytes"
	"github.com/BurntSushi/toml"
	"log"
)

func ToTomlConfig(val interface{}) string {
	buf := bytes.Buffer{}
	enc := toml.NewEncoder(&buf)
	err := enc.Encode(val)
	if err != nil {
		log.Panicf("Encode to a valid TOML config fails because of %v", err)
	}
	return buf.String()
}
