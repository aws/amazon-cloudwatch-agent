// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8smetadata

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct{}

var _ component.Config = (*Config)(nil)
