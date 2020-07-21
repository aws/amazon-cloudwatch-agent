// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers

import (
	"github.com/aws/amazon-cloudwatch-agent/awscsm/csm"
)

// Key type definitions
var apiDefinition = EventEntryDefinition{
	Name:    "Api",
	KeyType: csm.MonitoringEventEntryKeyTypeAggregation,
}

var clientIDDefinition = EventEntryDefinition{
	Name:    "ClientId",
	KeyType: csm.MonitoringEventEntryKeyTypeAggregation,
}

var serviceDefinition = EventEntryDefinition{
	Name:    "Service",
	KeyType: csm.MonitoringEventEntryKeyTypeAggregation,
}

var timestampDefinition = EventEntryDefinition{
	Name:    "Timestamp",
	KeyType: csm.MonitoringEventEntryKeyTypeAggregationTimestamp,
}

var typeDefinition = EventEntryDefinition{
	Name:    "Type",
	KeyType: csm.MonitoringEventEntryKeyTypeAggregation,
}

var regionDefinition = EventEntryDefinition{
	Name:    "Region",
	KeyType: csm.MonitoringEventEntryKeyTypeAggregation,
}

var userAgentDefinition = EventEntryDefinition{
	Name:    "UserAgent",
	KeyType: csm.MonitoringEventEntryKeyTypeAggregation,
}

// Event monitoring frequency definitions

var attemptCountDefinition = EventEntryDefinition{
	Name: "AttemptCount",
	Type: csm.MonitoringEventEntryMetricTypeFrequency,
}

var maxRetriesExceededDefinition = EventEntryDefinition{
	Name: "MaxRetriesExceeded",
	Type: csm.MonitoringEventEntryMetricTypeFrequency,
}

var apiCallTimeoutDefinition = EventEntryDefinition{
	Name: "ApiCallTimeout",
	Type: csm.MonitoringEventEntryMetricTypeFrequency,
}

var httpStatusCodeDefinition = EventEntryDefinition{
	Name:    "HttpStatusCode",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
	Type:    csm.MonitoringEventEntryMetricTypeFrequency,
}

var finalHttpStatusCodeDefinition = EventEntryDefinition{
	Name:    "FinalHttpStatusCode",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
	Type:    csm.MonitoringEventEntryMetricTypeFrequency,
}

var sdkExceptionDefinition = EventEntryDefinition{
	Name: "SdkException",
	Type: csm.MonitoringEventEntryMetricTypeFrequency,
}

var finalSdkExceptionDefinition = EventEntryDefinition{
	Name: "FinalSdkException",
	Type: csm.MonitoringEventEntryMetricTypeFrequency,
}

var awsExceptionDefinition = EventEntryDefinition{
	Name: "AwsException",
	Type: csm.MonitoringEventEntryMetricTypeFrequency,
}

var finalAwsExceptionDefinition = EventEntryDefinition{
	Name: "FinalAwsException",
	Type: csm.MonitoringEventEntryMetricTypeFrequency,
}

var destinationIPDefinition = EventEntryDefinition{
	Name:    "DestinationIp",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
	Type:    csm.MonitoringEventEntryMetricTypeFrequency,
}

var connectionReusedDefinition = EventEntryDefinition{
	Name:    "ConnectionReused",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
	Type:    csm.MonitoringEventEntryMetricTypeFrequency,
}

// Event monitoring SEH definitions

var acquireConnectionLatencyDefinition = EventEntryDefinition{
	Name: "AcquireConnectionLatency",
	Type: csm.MonitoringEventEntryMetricTypeSeh,
}

var attemptLatencyDefinition = EventEntryDefinition{
	Name: "AttemptLatency",
	Type: csm.MonitoringEventEntryMetricTypeSeh,
}

var connectLatencyDefinition = EventEntryDefinition{
	Name: "ConnectLatency",
	Type: csm.MonitoringEventEntryMetricTypeSeh,
}

var dnsLatencyDefinition = EventEntryDefinition{
	Name: "DnsLatency",
	Type: csm.MonitoringEventEntryMetricTypeSeh,
}

var latencyDefinition = EventEntryDefinition{
	Name: "Latency",
	Type: csm.MonitoringEventEntryMetricTypeSeh,
}

var requestLatencyDefinition = EventEntryDefinition{
	Name: "RequestLatency",
	Type: csm.MonitoringEventEntryMetricTypeSeh,
}

var sslLatencyDefinition = EventEntryDefinition{
	Name: "SslLatency",
	Type: csm.MonitoringEventEntryMetricTypeSeh,
}

var tcpLatencyDefinition = EventEntryDefinition{
	Name: "TcpLatency",
	Type: csm.MonitoringEventEntryMetricTypeSeh,
}

// sample definitions

var fqdnDefinition = EventEntryDefinition{
	Name:    "Fqdn",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
}

var sessionTokenDefinition = EventEntryDefinition{
	Name:    "SessionToken",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
}

var akidDefinition = EventEntryDefinition{
	Name:    "AccessKey",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
}

var awsExceptionMessageDefinition = EventEntryDefinition{
	Name:    "AwsExceptionMessage",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
}

var finalAwsExceptionMessageDefinition = EventEntryDefinition{
	Name:    "FinalAwsExceptionMessage",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
}

var sdkExceptionMessageDefinition = EventEntryDefinition{
	Name:    "SdkExceptionMessage",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
}

var finalSdkExceptionMessageDefinition = EventEntryDefinition{
	Name:    "FinalSdkExceptionMessage",
	KeyType: csm.MonitoringEventEntryKeyTypeSample,
}
