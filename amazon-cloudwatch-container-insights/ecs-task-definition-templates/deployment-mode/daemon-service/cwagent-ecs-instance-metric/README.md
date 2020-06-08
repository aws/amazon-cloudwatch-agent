## CloudWatch Agent for ECS Instance Metrics

The sample ECS task definitions in this folder deploy the CloudWatch Agent as a DaemonService. For more information, see [Setting Up Container Insights on Amazon ECS](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/deploy-container-insights-ECS.html).

* [cwagent-ecs-instance-metric.json](cwagent-ecs-instance-metric.json): sample ECS task definition

You must replace all the placeholders (with ```{{ }}```) in the above task definitions with your information:
* ```{{task-role-arn}}```: ECS task role ARN.
  * This is role that you application containers will use. The permission should be whatever your applications need.
  * Additionally, ensure that the ```CloudWatchAgentServerPolicy``` policy is attached to your ECS Task Role.
  
* ```{{execution-role-arn}}```: ECS task execution role ARN.
  * This is the role that Amazon ECS requires to launch/execute your containers, e.g. get the parameters from SSM parameter store.
  * Ensure that the ```AmazonSSMReadOnlyAccess```, ```AmazonECSTaskExecutionRolePolicy``` and ```CloudWatchAgentServerPolicy``` policies are attached to your ECS Task execution role.
  * If you would like to store more sensitive data for ECS to use, refer to https://docs.aws.amazon.com/AmazonECS/latest/developerguide/specifying-sensitive-data.html.    

* ```{{awslogs-region}}```: The AWS region where the container logs should be published: e.g. ```us-west-2```

You can also adjust the resource limit (e.g. cpu and memory) based on your particular use cases.