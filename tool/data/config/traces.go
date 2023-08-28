// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

type Traces struct {
	TracesCollected struct {
		Xray struct {
			BindAddress string `json:"bind_address"`
			TcpProxy    struct {
				BindAddress string `json:"bind_address"`
			} `json:"tcp_proxy"`
		} `json:"xray"`
	} `json:"traces_collected"`
	Concurrency  int    `json:"concurrency"`
	BufferSizeMB int    `json:"buffer_size_mb"`
	ResourceArn  string `json:"resource_arn,omitempty"`
	LocalMode    bool   `json:"local_mode,omitempty"` //local
	Insecure     bool   `json:"insecure, omitempty"`  //noverifyssl
	Credentials  *struct {
		RoleArn string `json:"role_arn,omitempty"`
	} `json:"credentials,omitempty"`
	EndpointOverride string `json:"endpoint_override,omitempty"` //endpoint
	RegionOverride   string `json:"region_override,omitempty"`   //region
	ProxyOverride    string `json:"proxy_override,omitempty"`
}

func (config *Traces) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {

	jsonData, _ := json.Marshal(config)
	var resultMap map[string]interface{}
	err := json.Unmarshal(jsonData, &resultMap)
	//if failure then the user's config was incorrect
	if err != nil {
		return "", nil
	}
	return "traces", resultMap

}
