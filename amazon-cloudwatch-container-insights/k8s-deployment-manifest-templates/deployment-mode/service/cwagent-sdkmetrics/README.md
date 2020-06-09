## CloudWatch Agent for AWS SDK Metrics

* [cwagent-sdkmetrics.yaml](cwagent-sdkmetrics.yaml) deploys the CloudWatch Agent as a service, and enables AWS SDK Metrics. For more information, see [Monitor Applications Using AWS SDK Metrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-SDK-Metrics.html).

### IAM permissions required by CloudWatch Agent for this functionality:
* CloudWatchAgentServerPolicy
* [Custom AmazonSDKMetrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Set-IAM-Permissions-For-SDK-Metrics.html)

### Configure Environment Variables in Your Application
You must configure your application to communicate with SDK Metrics by setting environment variables.

To configure your application to talk to SDK Metrics, you need to set some environment variables. These are read by the AWS SDK, and tell the SDK how to talk to the CloudWatch agent. 

|Variable        |Description                                                                                     |Value                                  |
|----------------|------------------------------------------------------------------------------------------------|---------------------------------------|
|AWS_CSM_ENABLED |Set this to true to enable SDK Metrics                                                          |true                                   |
|AWS_CSM_PORT    |The port to send metrics to                                                                     |31000                                  |
|AWS_CSM_HOST    |The host to send metrics to. This is required only when running your application in a container |cloudwatch-agent.amazon-cloudwatch.svc |

### Using VPC Security Groups to Enhance Security
Because SDK Metrics sends plain-text UDP datagrams to the CloudWatch agent, we recommend that you use security groups to control access to the CloudWatch agent. For more information, see [Security Groups for Your VPC](https://alpha-docs-aws.amazon.com/vpc/latest/userguide/VPC_SecurityGroups.html).

Since you deploy the CloudWatch agent as a service, your application talks to the agent over the network. You should configure the security groups to allow your application to send traffic to the CloudWatch agent. 