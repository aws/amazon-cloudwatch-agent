## Sample AWS CloudFormation Template for Redis Application
This directory contains the sample AWS CloudFormation template for Redis application to be installed on the ECS Fargate cluster.

Installation Steps:
* Setup ENV Variables
```
ECS_CLUSTER_NAME=your_target_ecs_fargate_cluster_name
AWS_DEFAULT_REGION=your_ecs_cluster_region_eg_ca-central-1
ECS_CLUSTER_SUBNET=your_ecs_cluster_subnet_id_eg_subnet-xxxxxxx
ECS_CLUSTER_SECURITY_GROUP=your_security_group_id_eg_sg-xxxxxxx
ECS_TASK_ROLE_NAME=redis-prometheus-demo-ecs-task-role-name
ECS_EXECUTION_ROLE_NAME=redis-prometheus-demo-ecs-execution-role-name
```
* Create AWS CloudFormation Stack
Run the following command to install the Sample Redis Application on Amazon EKS or Kubernetes
```
aws cloudformation create-stack --stack-name Redis-Prometheus-Demo-ECS-$ECS_CLUSTER_NAME-fargate-awsvpc \
    --template-body file://redis-traffic-sample.yaml \
    --parameters ParameterKey=ECSClusterName,ParameterValue=$ECS_CLUSTER_NAME \
                 ParameterKey=SecurityGroupID,ParameterValue=$ECS_CLASTER_SECURITY_GROUP \
                 ParameterKey=SubnetID,ParameterValue=$ECS_CLUSTER_SUBNET \
                 ParameterKey=TaskRoleName,ParameterValue=$ECS_TASK_ROLE_NAME \
                 ParameterKey=ExecutionRoleName,ParameterValue=$ECS_EXECUTION_ROLE_NAME \
    --capabilities CAPABILITY_NAMED_IAM \
    --region $AWS_DEFAULT_REGION
```
