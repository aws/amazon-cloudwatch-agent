// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package transformprocessor

import _ "embed"

//go:embed transform_logs_routing_host_windows.yaml
var transformLogsRoutingHostConfig string
