// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package config

import _ "embed"

//go:embed defaults/otel_windows.json
var defaultOtelConfig string
