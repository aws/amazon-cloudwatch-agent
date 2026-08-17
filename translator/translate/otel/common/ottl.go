// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

// OTTL error modes control how processors handle statement evaluation failures.
// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor#config
const (
	OTTLErrorModeIgnore    = "ignore"
	OTTLErrorModePropagate = "propagate"
)
