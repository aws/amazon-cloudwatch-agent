## Example Kubernetes YAML files for Daemonset deployment mode

This folder contains the example Kubernetes YAML files for Daemonset deployment mode.

Check the subfolders for the functionality you want:

### [container-insights-monitoring](container-insights-monitoring)
This folder provides the functionality that enables you to deploy [Container Insights](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-EKS-quickstart.html). It provides the CloudWatch Agent and Fluentd as a Daemonset separately to collect metrics and logs.

### [cwagent-statsd](cwagent-statsd)
This folder provides the functionality that enables you to utilize `StatsD`. It provides the CloudWatch Agent as a Daemonset to receive `StatsD` metrics.

### [cwagent-fluentd-xray](cwagent-fluentd-xray)
This folder provides a way to setup CloudWatch Agent, FluentD and XRay Daemon in one commandline.

### [combination](combination)
This folder provides an example about how to deploy the combination of all individual functionality defined in this directory, in Daemonset deployment mode.


