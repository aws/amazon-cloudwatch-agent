## Sample CloudWatch Dashboard for Nginx-Igress Prometheus Metrics
Please refer to [Viewing Your Prometheus Metrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-viewmetrics.html) for details.

### Usage Guide

#### Required Permission
You need the following permission to create a dashboard or update an existing dashboard.
```
cloudwatch:PutDashboard
```

#### Setup Dashboard Variables
Replace the values below to match your setup

```
DASHBOARD_NAME=your_cw_dashboard_name
REGION_NAME=us-east-1
CLUSTER_NAME=your_k8s_cluster_name_here
```

#### Create Dashboard
Create the CloudWatch Dashboard by the AWS CLI command as below:
```
cat cw_dashboard_nginx_ingress_controller.json \
| sed "s/{{YOUR_AWS_REGION}}/${REGION_NAME}/g" \
| sed "s/{{YOUR_CLUSTER_NAME}}/${CLUSTER_NAME}/g" \
| xargs -0 aws cloudwatch put-dashboard --dashboard-name ${DASHBOARD_NAME} --dashboard-body
```
