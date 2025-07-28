// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"regexp"
)

type DiagnosticSuggestion struct {
	Pattern     *regexp.Regexp
	Issue       string
	Possibility string
}

// Metric related errors
var metricErrors = []DiagnosticSuggestion{
	{
		regexp.MustCompile(`E! cloudwatch: code: AccessDenied.*is not authorized to perform: cloudwatch:PutMetricData`),
		"Problem: CloudWatch Agent is trying to push metrics via the PutMetricData endpoint but it does not have necessary permissions to do so.",
		"Fix: Your IAM role requires 'cloudwatch:PutMetricData' action allowed. Add CloudWatchCustomServerPolicy to your IAM role or if you are using a custom role add 'cloudwatch:PutMetricData'.",
	},
	{
		regexp.MustCompile(`E!.*cloudwatch.*ThrottlingException.*Rate exceeded.*PutMetricData`),
		"Problem: CloudWatch Agent is being throttled by AWS due to exceeding API rate limits for the PutMetricData endpoint. This can result in increased memory usage.",
		"Fix: Reduce the frequency or volume of metrics being sent. If throttling persists, consider requesting a service quota increase from AWS.",
	},
}

// Log related errors
var logErrors = []DiagnosticSuggestion{
	{
		regexp.MustCompile(`E!.*\[outputs\.cloudwatchlogs\].*AccessDeniedException.*logs:PutLogEvents`),
		"Problem: CloudWatch Agent is trying to push logs via the logs:PutLogEvents endpoint but it does not have necessary permissions to do so.",
		"Fix: Your IAM role requires 'logs:PutLogEvents' action allowed. Add CloudWatchCustomServerPolicy to your IAM role. If you are using a custom role add 'logs:PutLogEvents'.",
	},
	{
		regexp.MustCompile(`E!.*\[outputs\.cloudwatchlogs\].*AccessDeniedException.*logs:DescribeLogGroups`),
		"Problem: CloudWatch Agent is trying to read log groups via the logs:DescribeLogGroups endpoint but it does not have necessary permissions to do so.",
		"Fix: Your IAM role requires 'logs:DescribeLogGroups' action allowed. Add CloudWatchAgentServerPolicy to your IAM role. If you are using a custom role add 'logs:DescribeLogGroups'.",
	},
	{
		regexp.MustCompile(`E!.*\[outputs\.cloudwatchlogs\].*AccessDeniedException.*logs:CreateLogStream`),
		"Problem: CloudWatch Agent is trying to create a log stream via the logs:CreateLogStream endpoint but it does not have necessary permissions to do so.",
		"Fix: Your IAM role requires 'logs:CreateLogStream' action allowed. Add CloudWatchAgentServerPolicy to your IAM role. If you are using a custom role add 'logs:CreateLogStream'.",
	},
	{
		regexp.MustCompile(`E!.*\[outputs\.cloudwatchlogs\].*AccessDeniedException.*logs:CreateLogGroup`),
		"Problem: CloudWatch Agent is trying to create a log group via the logs:CreateLogGroup endpoint but it does not have necessary permissions to do so.",
		"Fix: Your IAM role requires 'logs:CreateLogGroup' action allowed. Add CloudWatchAgentServerPolicy to your IAM role. If you are using a custom role add 'logs:CreateLogGroup'.",
	},
	{
		regexp.MustCompile(`E!.*\[outputs\.cloudwatchlogs\].*failed to update retention policy.*after \d+ attempts`),
		"Problem: CloudWatch Agent is trying to set the retention of a log group via the logs:PutRetentionPolicy endpoint but it does not have necessary permissions to do so.",
		"Fix: Your IAM role requires 'logs:PutRetentionPolicy' action allowed. Add CloudWatchAgentServerPolicy to your IAM role. If you are using a custom role add 'logs:PutRetentionPolicy'.",
	},
	{
		regexp.MustCompile(`E!.*cloudwatch.*ThrottlingException.*Rate exceeded.*(PutLogEvents|DescribeLogGroups|CreateLogStream|CreateLogGroup|PutRetentionPolicy)`),
		"Problem: CloudWatch Agent is being throttled by AWS due to exceeding API rate limits for the CloudWatchLogs service.",
		"Fix: Reduce the frequency or volume of logs being sent. If throttling persists, consider requesting a service quota increase from AWS.",
	},
}

// Trace related errors
var traceErrors = []DiagnosticSuggestion{
	{
		regexp.MustCompile(`E!.*AccessDeniedException.*not authorized.*xray:PutTraceSegments`),
		"Problem: CloudWatch Agent is trying to push traces via the xray:PutTraceSegments endpoint but it does not have necessary permissions to do so.",
		"Fix: Your IAM role requires 'xray:PutTraceSegments' action allowed. Add CloudWatchAgentServerPolicy to your IAM role. If you are using a custom role add 'xray:PutTraceSegments'.",
	},
}

// Credential related errors
var credentialErrors = []DiagnosticSuggestion{
	{
		regexp.MustCompile(`E!.*(code: NoCredentialProviders|NoCredentialProviders).*no valid providers in chain`),
		"Problem: The agent was not able to find any usable AWS credentials. The default credential provider chain failed to locate credentials from the environment, EC2 instance profile, or configuration files.",
		"Fix: Attach an IAM role to the EC2 instance with the necessary permissions. Verify that the Instance Metadata Service (IMDS) is enabled and reachable so the agent can retrieve credentials from the instance profile. If you're using environment variables or AWS config files to provide credentials, verify they are set correctly in the agent's runtime environment.",
	},
}

// Network related errors
var networkErrors = []DiagnosticSuggestion{
	{
		regexp.MustCompile(`dial tcp 169\.254\.169\.254:80: i/o timeout`),
		"Problem: IMDS can't be accessed/is blocked on your instance.",
		"Fix: Ensure that the Instance Metadata Service (IMDS) is enabled for the instance, and that IMDSv2 settings are compatible with the agent configuration. Also check that the instance has an attached IAM role, and that no local firewall or network config is blocking access to 169.254.169.254.",
	},
	{
		regexp.MustCompile(`proxyconnect tcp: dial tcp .*: connect: connection refused`),
		"Problem: CWAgent is configured to use a proxy but is unable to connect to the proxy server. This prevents the agent from reaching AWS services such as CloudWatch, CloudWatch Logs or the IMDS.",
		"Fix: Ensure proxy is correctly configured and reachable. Also, check if the AWS endpoints are reachable through your proxy.",
	},
	{
		regexp.MustCompile(`lookup .*\.amazonaws\.com.*: no such host`),
		"Problem: CWAgent failed to resolve an AWS service hostname. This often happens when a custom endpoint override is misconfigured or if DNS resolution is blocked or unavailable on the instance.",
		"Fix: If using a custom endpoint_override, verify that the hostname is correct and resolvable. Otherwise, check your instance's DNS configuration, VPC settings, and ensure outbound DNS (UPD port 53) is not blocked.",
	},
	{
		regexp.MustCompile(`dial tcp 10\..*:443: connect: connection refused`),
		"Problem: CWAgent is trying to connect to a VPC interface endpoint but the connection was refused. This usually means the endpoint is not responding, or security group rules are preventing the connection.",
		"Fix: Verify that all required VPC interface endpoints are available and properly configured. Check that the endpoints are associated with subnets in the same Availability Zone as the instance, and that security groups and network ACLs allow inbound HTTPS (TCP 443) traffic from the instance.",
	},
	{
		regexp.MustCompile(`dial tcp .*:443: i/o timeout`),
		"Problem: CWAgent is unable to reach AWS service endpoints over HTTPS (TCP port 443). This is required for sending metrics, logs, and performing API calls to services like CloudWatch, EC2, and SSM. The connection timeout suggests outbound traffic is being blocked.",
		"Fix: Check if a local or upstream firewall, network ACL, or proxy is blocking outbound HTTPS (TCP port 443) to AWS endpoints. Also verify DNS and proxy settings if applicable.",
	},
}

func InitializeCommonErrors() []DiagnosticSuggestion {
	allPatterns := []DiagnosticSuggestion{}
	allPatterns = append(allPatterns, metricErrors...)
	allPatterns = append(allPatterns, logErrors...)
	allPatterns = append(allPatterns, traceErrors...)
	allPatterns = append(allPatterns, credentialErrors...)
	allPatterns = append(allPatterns, networkErrors...)

	return allPatterns
}
