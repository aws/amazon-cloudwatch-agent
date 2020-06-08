## CloudWatch Agent for AWS SDK Metrics

The sample Amazon ECS task definitions in this folder deploy the CloudWatch Agent as a Sidecar to your application to enable AWS SDK Metrics. For more information, see [Monitor Applications Using AWS SDK Metrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-SDK-Metrics.html).

* [cwagent-sdkmetrics-ec2.json](cwagent-sdkmetrics-ec2.json): sample ECS task definition for EC2 launch type.
* [cwagent-sdkmetrics-fargate.json](cwagent-sdkmetrics-fargate.json): sample ECS task definition for Fargate launch type.

The main difference between the above 2 task definitions are around the ```networkMode```: ```bridge``` vs ```awsvpc```.

To configure your application to talk to SDK Metrics, you need to set some environment variables. These are read by the AWS SDK, and tell the SDK how to talk to the CloudWatch agent. 

|Variable        |Description                                                                                     |Value                                |
|----------------|------------------------------------------------------------------------------------------------|-------------------------------------|
|AWS_CSM_ENABLED |Set this to true to enable SDK Metrics                                                          |true                                 |
|AWS_CSM_PORT    |The port to send metrics to                                                                     |31000                                |
|AWS_CSM_HOST    |The host to send metrics to. This is required only when running your application in a container |127.0.0.1 (fargate) or 0.0.0.0 (ec2) |


You must replace all the placeholders (with ```{{ }}```) in the above task definitions with your information:
* ```{{task-role-arn}}```: ECS task role ARN.
  * This is role that you application containers will use. The permission should be whatever your applications need.
  * Additionally, ensure that the ```CloudWatchAgentServerPolicy``` and [Custom AmazonSDKMetrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Set-IAM-Permissions-For-SDK-Metrics.html) policies are attached to your ECS Task Role.

* ```{{execution-role-arn}}```: ECS task execution role ARN.
  * This is the role that Amazon ECS requires to launch/execute your containers, e.g. get the parameters from SSM parameter store.
  * Ensure that the ```AmazonSSMReadOnlyAccess```, ```AmazonECSTaskExecutionRolePolicy``` and ```CloudWatchAgentServerPolicy``` policies are attached to your ECS Task execution role.
  * If you would like to store more sensitive data for ECS to use, refer to https://docs.aws.amazon.com/AmazonECS/latest/developerguide/specifying-sensitive-data.html.    

* ```{{awslogs-region}}```: The AWS region where the container logs should be published: e.g. ```us-west-2```

You can also adjust the resource limit (e.g. cpu and memory) based on your particular use cases.

Configure the CloudWatch agent through [SSM parameter](https://docs.aws.amazon.com/systems-manager/latest/userguide/sysman-paramstore-su-create.html).

If you are launching an ECS task for the EC2 launch type, please create a SSM parameter ```ecs-cwagent-sidecar-ec2``` in the region where your cluster is located, with the following text:
```
{
  "csm": {
    "service_addresses": ["udp4://0.0.0.0:31000", "udp6://[::]:31000"],
    "memory_limit_in_mb": 20
  }
}
```

If you are launching an ECS task for the Fargate launch type, please create a SSM parameter ```ecs-cwagent-sidecar-fargate``` in the region where your cluster is located, with the following text:
```
{
  "csm": {
    "service_addresses": ["udp4://127.0.0.1:31000", "udp6://[::1]:31000"],
    "memory_limit_in_mb": 20
  }
}
```