// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package config

import _ "embed"

//go:embed defaults/otel.json
var defaultOtelConfig string
