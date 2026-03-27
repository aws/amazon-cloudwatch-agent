// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsneuron

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct{}

// Verify Config implements component.Config interface.
var _ component.Config = (*Config)(nil)
