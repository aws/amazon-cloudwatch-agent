## SDK Metrics
* [cwagent-sdkmetrics.yaml](cwagent-sdkmetrics.yaml) deploys the CloudWatch Agent as a sidecar to enable CloudWatch SDK Metrics. For more information, see [Monitor Applications Using AWS SDK Metrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-SDK-Metrics.html).

### IAM permissions required by CloudWatch Agent for this functionality:
* CloudWatchAgentServerPolicy
* [Custom AmazonSDKMetrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Set-IAM-Permissions-For-SDK-Metrics.html)

### Configure Environment Variables in Your Application
To configure your application to talk to SDK Metrics, you need to set some environment variables. These are read by the AWS SDK, and tell the SDK how to talk to the CloudWatch agent. 

|Variable        |Description                                                                                     |Value     |
|----------------|------------------------------------------------------------------------------------------------|----------|
|AWS_CSM_ENABLED |Set this to true to enable SDK Metrics                                                          |true      |
|AWS_CSM_PORT    |The port to send metrics to                                                                     |31000     |
|AWS_CSM_HOST    |The host to send metrics to. This is required only when running your application in a container |127.0.0.1 |

### Using VPC Security Groups to Enhance Security
Since you deploy the CloudWatch agent as a sidecar, your application talks to the agent within the same container. Data does not leave the node. Security groups are not required, as the CloudWatch agent should be configured to listen only on the loopback address. 