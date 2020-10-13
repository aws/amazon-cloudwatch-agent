## Sample CloudWatch Dashboard for Memcached Prometheus Metrics

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
REGION_NAME=your_aws_region_eg:us-east-1
CLUSTER_NAME=your_ecs_cluster_name_here
ECS_TASK_DEF_FAMILY=your_memcached_ecs_task definition_family_name_here
```

#### Create Dashboard
Create the CloudWatch Dashboard by the AWS CLI command as below:
```
cat cw_dashboard_memcached.json \
| sed "s/{{YOUR_AWS_REGION}}/${REGION_NAME}/g" \
| sed "s/{{YOUR_CLUSTER_NAME}}/${CLUSTER_NAME}/g" \
| sed "s/{{YOUR_TASK_DEF_FAMILY}}/${ECS_TASK_DEF_FAMILY}/g" \
| xargs -0 aws cloudwatch put-dashboard --dashboard-name ${DASHBOARD_NAME} --region $REGION_NAME --dashboard-body
```
