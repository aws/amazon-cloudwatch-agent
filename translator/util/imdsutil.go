// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"os"
	"strconv"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func LoadImdsRetries(imdsConfig *commonconfig.IMDS) {
	if imdsConfig != nil && imdsConfig.ImdsRetries != nil && *imdsConfig.ImdsRetries >= 0 {
		_ = os.Setenv(envconfig.IMDS_NUMBER_RETRY, strconv.Itoa(*imdsConfig.ImdsRetries))
	}
}
