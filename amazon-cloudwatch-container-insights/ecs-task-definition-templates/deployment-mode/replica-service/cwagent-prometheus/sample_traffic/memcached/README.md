## Sample AWS CloudFormation Template for Memcached Application
This directory contains the AWS CloudFormation template for a demo Memcached application to be installed on the ECS EC2 cluster.
The workload can be installed in either `host` or `bridge` network mode. 

Installation Steps:
* Setup ENV Variables
```shell script
ECS_CLUSTER_NAME=your_target_ecs_fargate_cluster_name
AWS_DEFAULT_REGION=your_ecs_cluster_region_eg_ca-central-1
ECS_NETWORK_MODE=host_or_bridge
ECS_TASK_ROLE_NAME=memcached-prometheus-demo-ecs-task-role-name
ECS_EXECUTION_ROLE_NAME=memcached-prometheus-demo-ecs-execution-role-name
```

* Create AWS CloudFormation Stack
Run the following command to install the Sample Memcached Application on Amazon ECS EC2 cluster
```shell script
aws cloudformation create-stack --stack-name Memcached-Prometheus-Demo-ECS-$ECS_CLUSTER_NAME-EC2-$ECS_NETWORK_MODE \
    --template-body file://memcached-traffic-sample.yaml \
    --parameters ParameterKey=ECSClusterName,ParameterValue=$ECS_CLUSTER_NAME \
                 ParameterKey=ECSNetworkMode,ParameterValue=$ECS_NETWORK_MODE \
                 ParameterKey=TaskRoleName,ParameterValue=$ECS_TASK_ROLE_NAME \
                 ParameterKey=ExecutionRoleName,ParameterValue=$ECS_EXECUTION_ROLE_NAME \
    --capabilities CAPABILITY_NAMED_IAM \
    --region $AWS_DEFAULT_REGION
```
