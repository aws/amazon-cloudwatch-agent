// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package awscloudwatchlogsprovisionerextension implements extensionauth.HTTPClient
// to dynamically set x-aws-log-group headers and create CloudWatch log groups and
// streams on first encounter.
package awscloudwatchlogsprovisionerextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscloudwatchlogsprovisionerextension"
