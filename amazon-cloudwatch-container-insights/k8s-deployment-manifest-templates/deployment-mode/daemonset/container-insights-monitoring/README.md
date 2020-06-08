## CloudWatch Agent for Container Insights Kubernetes Monitoring

* [cwagent](cwagent) provides the functionality that enables you to collect [Container Insights Metrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-metrics.html). It provides CloudWatch Agent as a Daemonset to collect Kubernetes metrics.
* [fluentd](fluentd) provides the functionality that enables you to collect [Container Insights Logs](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-logs.html). It provides Fluentd as a Daemonset to collect Kubernetes logs.
* [quickstart](quickstart) provides a way to setup Container Insights using only [one command line](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-EKS-quickstart.html). 


### IAM permissions required by CloudWatch Agent and Fluentd for this functionality:
* CloudWatchAgentServerPolicy