// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"os"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func GetSSL(sslConfig map[string]string) (result map[string]string) {
	result = make(map[string]string)
	if val, ok := sslConfig[commonconfig.CABundlePath]; ok {
		result[commonconfig.CABundlePath] = val
		return
	}
	names := []string{envconfig.AWS_CA_BUNDLE}
	for _, name := range names {
		if val := os.Getenv(name); val != "" {
			result[commonconfig.CABundlePath] = val
			return
		}
	}
	return
}

func SetSSLEnv(sslConfig map[string]string) {
	if ssl := GetSSL(sslConfig); len(sslConfig) > 0 {
		os.Setenv(envconfig.AWS_CA_BUNDLE, ssl[commonconfig.CABundlePath])
	}

}
