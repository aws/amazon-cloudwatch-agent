## CloudWatch Agent for Prometheus Metrics Quick Start

The cloud formation templates in this folder help you to quickly deploy CloudWatch Agent as replica-service to discover and collect Prometheus metrics.

### Supported Matrix
CloudWatch Agent with Prometheus Monitoring can be deployed in the following modes

|ECS Launch Type         | ECS Network Mode    |
|------------------------|---------------------|
|EC2 (Linux)             | bridge              |
|EC2 (Linux)             | host                |
|EC2 (Linux)             | awsvpc              |
|FARGATE                 | awsvpc              |

### CloudFormation Template Parameters
* **ECSClusterName**: specify the target ECS cluster for the installation
* **CreateIAMRoles**: whether to create ECS task role and ECS execution role or reusing existing ones. Either `True` or `False`
* **TaskRoleName**: the ECS task role name to be created when `CreateIAMRoles=True` or the ECS task role name to be reused when `CreateIAMRoles=False`
* **ExecutionRoleName**: the ECS execution role name to be created when `CreateIAMRoles=True` or the ECS execution role name to be reused when `CreateIAMRoles=False`
* **ECSNetworkMode**: specify the ECS task network mode when the ECS cluster launch type is `EC2`, either `bridge` or `host`
* **ECSLaunchType**: specify the ECS cluster launch type when the network mode is `awsvpc`, either `FARGATE` or `EC2`
* **SecurityGroupID**: specify the security group ID when ECS task network mode is `awsvpc`
* **SubnetID**: specify the subnet ID when ECS task network mode is `awsvpc`

### Samples Commands
##### Create AWS CloudFormation Stack for `EC2` ECS Cluster with `bridge` network mode

```
export AWS_PROFILE=your_aws_config_profile_eg_default
export AWS_DEFAULT_REGION=your_aws_region_eg_ap-southeast-1
export ECS_CLUSTER_NAME=your_ec2_ecs_cluster_name
export ECS_NETWORK_MODE=bridge
export CREATE_IAM_ROLES=True

aws cloudformation create-stack --stack-name CWAgent-Prometheus-ECS-${ECS_CLUSTER_NAME}-EC2-${ECS_NETWORK_MODE} \
    --template-body file://cwagent-ecs-prometheus-metric-for-bridge-host.yaml \
    --parameters ParameterKey=ECSClusterName,ParameterValue=${ECS_CLUSTER_NAME} \
                 ParameterKey=CreateIAMRoles,ParameterValue=${CREATE_IAM_ROLES} \
                 ParameterKey=ECSNetworkMode,ParameterValue=${ECS_NETWORK_MODE} \
                 ParameterKey=TaskRoleName,ParameterValue=CWAgent-Prometheus-TaskRole-${ECS_CLUSTER_NAME} \
                 ParameterKey=ExecutionRoleName,ParameterValue=CWAgent-Prometheus-ExecutionRole-${ECS_CLUSTER_NAME} \
    --capabilities CAPABILITY_NAMED_IAM \
    --region ${AWS_DEFAULT_REGION} \
    --profile ${AWS_PROFILE}
```

##### Create AWS CloudFormation Stack for `EC2` ECS Cluster with `host` network type

```
export AWS_PROFILE=your_aws_config_profile_eg_default
export AWS_DEFAULT_REGION=your_aws_region_eg_ap-southeast-1
export ECS_CLUSTER_NAME=your_ec2_ecs_cluster_name
export ECS_NETWORK_MODE=host
export CREATE_IAM_ROLES=True

aws cloudformation create-stack --stack-name CWAgent-Prometheus-ECS-${ECS_CLUSTER_NAME}-EC2-${ECS_NETWORK_MODE} \
    --template-body file://cwagent-ecs-prometheus-metric-for-bridge-host.yaml \
    --parameters ParameterKey=ECSClusterName,ParameterValue=${ECS_CLUSTER_NAME} \
                 ParameterKey=CreateIAMRoles,ParameterValue=${CREATE_IAM_ROLES} \
                 ParameterKey=ECSNetworkMode,ParameterValue=${ECS_NETWORK_MODE} \
                 ParameterKey=TaskRoleName,ParameterValue=CWAgent-Prometheus-TaskRole-${ECS_CLUSTER_NAME} \
                 ParameterKey=ExecutionRoleName,ParameterValue=CWAgent-Prometheus-ExecutionRole-${ECS_CLUSTER_NAME} \
    --capabilities CAPABILITY_NAMED_IAM \
    --region ${AWS_DEFAULT_REGION} \
    --profile ${AWS_PROFILE}
```

##### Create AWS CloudFormation Stack for `EC2` ECS Cluster with `awsvpc` network type

```
export AWS_PROFILE=your_aws_config_profile_eg_default
export AWS_DEFAULT_REGION=your_aws_region_eg_ap-southeast-1
export ECS_CLUSTER_NAME=your_ec2_ecs_cluster_name
export ECS_LAUNCH_TYPE=EC2
export CREATE_IAM_ROLES=True
export ECS_CLASTER_SECURITY_GROUP=your_security_group_eg_sg-xxxxxxxxxx
export ECS_CLUSTER_SUBNET=your_subnet_eg_subnet-xxxxxxxxxx

aws cloudformation create-stack --stack-name CWAgent-Prometheus-ECS-${ECS_CLUSTER_NAME}-${ECS_LAUNCH_TYPE}-awsvpc \
    --template-body file://cwagent-ecs-prometheus-metric-for-awsvpc.yaml \
    --parameters ParameterKey=ECSClusterName,ParameterValue=${ECS_CLUSTER_NAME} \
                 ParameterKey=CreateIAMRoles,ParameterValue=${CREATE_IAM_ROLES} \
                 ParameterKey=ECSLaunchType,ParameterValue=${ECS_LAUNCH_TYPE} \
                 ParameterKey=SecurityGroupID,ParameterValue=${ECS_CLASTER_SECURITY_GROUP} \
                 ParameterKey=SubnetID,ParameterValue=${ECS_CLUSTER_SUBNET} \
                 ParameterKey=TaskRoleName,ParameterValue=CWAgent-Prometheus-TaskRole-${ECS_CLUSTER_NAME} \
                 ParameterKey=ExecutionRoleName,ParameterValue=CWAgent-Prometheus-ExecutionRole-${ECS_CLUSTER_NAME} \
    --capabilities CAPABILITY_NAMED_IAM \
    --region ${AWS_DEFAULT_REGION} \
    --profile ${AWS_PROFILE}
```

##### Create AWS CloudFormation Stack for `FARGATE` ECS Cluster with `awsvpc` network type

```
export AWS_PROFILE=your_aws_config_profile_eg_default
export AWS_DEFAULT_REGION=your_aws_region_eg_ap-southeast-1
export ECS_CLUSTER_NAME=your_ec2_ecs_cluster_name
export ECS_LAUNCH_TYPE=FARGATE
export CREATE_IAM_ROLES=True
export ECS_CLASTER_SECURITY_GROUP=your_security_group_eg_sg-xxxxxxxxxx
export ECS_CLUSTER_SUBNET=your_subnet_eg_subnet-xxxxxxxxxx

aws cloudformation create-stack --stack-name CWAgent-Prometheus-ECS-${ECS_CLUSTER_NAME}-${ECS_LAUNCH_TYPE}-awsvpc \
    --template-body file://cwagent-ecs-prometheus-metric-for-awsvpc.yaml \
    --parameters ParameterKey=ECSClusterName,ParameterValue=${ECS_CLUSTER_NAME} \
                 ParameterKey=CreateIAMRoles,ParameterValue=${CREATE_IAM_ROLES} \
                 ParameterKey=ECSLaunchType,ParameterValue=${ECS_LAUNCH_TYPE} \
                 ParameterKey=SecurityGroupID,ParameterValue=${ECS_CLASTER_SECURITY_GROUP} \
                 ParameterKey=SubnetID,ParameterValue=${ECS_CLUSTER_SUBNET} \
                 ParameterKey=TaskRoleName,ParameterValue=CWAgent-Prometheus-TaskRole-${ECS_CLUSTER_NAME} \
                 ParameterKey=ExecutionRoleName,ParameterValue=CWAgent-Prometheus-ExecutionRole-${ECS_CLUSTER_NAME} \
    --capabilities CAPABILITY_NAMED_IAM \
    --region ${AWS_DEFAULT_REGION} \
    --profile ${AWS_PROFILE}
```

### AWS CloudFormation Resources Created

The following resources will be created by the cloud formation stack

|Resource Type           | Resource Name                                                                                | Comments                               |
|------------------------|----------------------------------------------------------------------------------------------|----------------------------------------|
|AWS::SSM::Parameter     | AmazonCloudWatch-CWAgentConfig-${ECSClusterName}-${ECSLaunchType}-${ECS_NETWORK_MODE}        |CloudWatch Agent with default AWS App Mesh and Java/Jmx EMF definition  |
|AWS::SSM::Parameter     | AmazonCloudWatch-PrometheusConfigName-${ECSClusterName}-${ECSLaunchType}-${ECS_NETWORK_MODE} |Prometheus scraping configuration       |
|AWS::IAM::Role          | CWAgent-Prometheus-TaskRole-${ECS_CLUSTER_NAME}                                              |created when CREATE_IAM_ROLES=True      |
|AWS::IAM::Role          | CWAgent-Prometheus-ExecutionRole-${ECS_CLUSTER_NAME}                                         |created when CREATE_IAM_ROLES=True      |
|AWS::ECS::Service       | cwagent-prometheus-replica-service-${ECSLaunchType}-${ECSNetworkMode}                        |                                        |
|AWS::ECS::TaskDefinition| cwagent-prometheus-${ECSClusterName}-${ECSLaunchType}-${ECS_NETWORK_MODE}                    |                                        |

