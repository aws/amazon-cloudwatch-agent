## CloudWatch Agent multi-feature deployments

* [combination.yaml](combination.yaml) provides an example about how to deploy Daemonset to achieve combination of all individual functionality defined in the parent directory.

* Note that there are some placeholders in the combination.yaml needs to be replaced before applying, you can find all placeholders by searching keyword "TODO:". In this file:
  * "{{region_name}}" should be replaced with the actual region to which your metrics/logs publish
  * "{{cluster-name}}" should be replaced with the actual name of your cluster
  
### IAM permissions required by CloudWatch Agent for all features:
* CloudWatchAgentServerPolicy
* AWSXRayDaemonWriteAccess
* [Custom AmazonSDKMetrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Set-IAM-Permissions-For-SDK-Metrics.html)