## CloudWatch Agent with Prometheus Monitoring support deployment yaml files

* [prometheus-eks.yaml](prometheus-eks.yaml) provides an example for deploying the CloudWatch Agent with Prometheus monitoring support for EKS in one command line:
```
kubectl apply -f https://raw.githubusercontent.com/aws-samples/amazon-cloudwatch-container-insights/prometheus-beta/k8s-deployment-manifest-templates/deployment-mode/service/cwagent-prometheus/prometheus-eks.yaml
```
* [prometheus-k8s.yaml](prometheus-k8s.yaml) provides an example for deploying the CloudWatch agent with Prometheus monitoring support for K8S on EC2 in one command line:
```
curl https://raw.githubusercontent.com/aws-samples/amazon-cloudwatch-container-insights/prometheus-beta/k8s-deployment-manifest-templates/deployment-mode/service/cwagent-prometheus/prometheus-k8s.yaml | sed "s/{{cluster_name}}/MyCluster/;s/{{region_name}}/region/" | kubectl apply -f -
```
Replace ```MyCluster``` with your cluster name, and ```region``` with the AWS region (e.g. ```us-west-2```).

### IAM permissions required by CloudWatch Agent for all features:
* CloudWatchAgentServerPolicy

### CloudWatch Agent Configuration:

#### CloudWatch Agent Prometheus Configuration:
CloudWatch Agent allows the customer to configure the Prometheus metrics setting in configuration map: `prometheus-cwagentconfig`.
For more information, see [Configuring the CloudWatch Agent for Prometheus Monitoring](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-Setup-configure.html).

#### Prometheus Scrape Configuration:
CloudWatch Agent allows the customer to specify a set of targets and parameters describing how to scrape them. The configuration is stored in configuration map: `prometheus-config`
The syntax is the same as [Prometheus Scrape Configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config)

### Default Prometheus Scrape Rules and EMF Metrics Configurations:
Both yaml files contain the default settings for the following containerized applications:

|Application     | Reference                                                                                                                 |
|----------------|---------------------------------------------------------------------------------------------------------------------------|
|NGINX_INGRESS   |Exposed by Helm Chart:   [stable/nginx-ingress](https://github.com/helm/charts/tree/master/stable/nginx-ingress)           |
|MEMCACHED       |Exposed by Helm Chart:   [stable/memcached](https://github.com/helm/charts/tree/master/stable/memcached)                   |
|HAPROXY_INGRESS |Exposed by Helm Chart:   [incubator/haproxy-ingress](https://github.com/helm/charts/tree/master/incubator/haproxy-ingress) |
|AWS APP MESH    |Exposed by Helm Chart:   [EKS Charts](https://github.com/aws/eks-charts/blob/master/README.md)                             |
|JAVA/JMX        |Exposed by JMX_Exporter: [JMX_Exporter](https://github.com/prometheus/jmx_exporter)                                        |
