// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import "github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"

const (
	RUN_IN_CONTAINER        = envconfig.RunInContainer
	RUN_IN_CONTAINER_TRUE   = envconfig.TrueValue
	RUN_IN_AWS              = envconfig.RunInAWS
	RUN_IN_AWS_TRUE         = envconfig.TrueValue
	RUN_WITH_IRSA           = envconfig.RunWithIRSA
	RUN_WITH_IRSA_TRUE      = envconfig.TrueValue
	USE_DEFAULT_CONFIG      = envconfig.UseDefaultConfig
	USE_DEFAULT_CONFIG_TRUE = envconfig.TrueValue
	HOST_NAME               = envconfig.HostName
	POD_NAME                = envconfig.PodName
	HOST_IP                 = envconfig.HostIP
	CWConfigContent         = envconfig.CWConfigContent
)
