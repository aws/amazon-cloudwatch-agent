## CloudWatch Agent, FluentD and XRay Daemon

* [cwagent-fluentd-xray-quickstart.yaml](cwagent-fluentd-xray-quickstart.yaml) provides a way to setup CloudWatch Agent, FluentD and XRay Daemon in one commandline:

```
curl https://raw.githubusercontent.com/aws-samples/amazon-cloudwatch-container-insights/master/k8s-deployment-manifest-templates/deployment-mode/daemonset/cwagent-fluentd-xray/cwagent-fluentd-xray-quickstart.yaml | sed "s/{{cluster_name}}/Cluster_Name/;s/{{region_name}}/Region/" | kubectl apply -f -
```

Replace ```Cluster_Name``` with your cluster name, and ```Region``` with the AWS region (e.g. ```us-west-2```).

### IAM permissions required by CloudWatch Agent, Fluentd and XRay Daemon for this functionality:
* CloudWatchAgentServerPolicy
* AWSXRayDaemonWriteAccess


