// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package extract

import "github.com/aws/amazon-cloudwatch-agent/internal/detector"

// platformPortExtractors returns an empty slice on non-Windows platforms.
// Linux SQL Server uses -p flag or MSSQL_TCP_PORT which are handled by the common extractors.
func platformPortExtractors() []detector.PortExtractor {
	return nil
}
