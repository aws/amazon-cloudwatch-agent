## Sample CloudWatch Dashboard for Redis Prometheus Metrics

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
REGION_NAME=your_metric_region_name_eg_us-east-1
CLUSTER_NAME=your_k8s_cluster_name_here
NAMESPACE=your_redis_service_namespace_here
```

#### Create Dashboard
Create the CloudWatch Dashboard by the AWS CLI command as below:
```
curl https://cwagent-prometheus-yamls-justin.s3-us-west-2.amazonaws.com/cw_dashboard_redis.json \
| sed "s/{{YOUR_AWS_REGION}}/${REGION_NAME}/g" \
| sed "s/{{YOUR_CLUSTER_NAME}}/${CLUSTER_NAME}/g" \
| sed "s/{{YOUR_NAMESPACE}}/${NAMESPACE}/g" \
| xargs -0 aws cloudwatch put-dashboard --dashboard-name ${DASHBOARD_NAME} --dashboard-body
```
