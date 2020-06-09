## CloudWatch Agent multi-feature deployments

* [combination.yaml](combination.yaml) provides an example for deploying the agent as a Sidecar to set up all the individual features.

### IAM permissions required by CloudWatch Agent for all features:
* CloudWatchAgentServerPolicy
* [Custom AmazonSDKMetrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Set-IAM-Permissions-For-SDK-Metrics.html)