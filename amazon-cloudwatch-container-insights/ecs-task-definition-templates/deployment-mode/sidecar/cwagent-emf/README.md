## CloudWatch Agent for Embedded Metric Format (EMF) Support

The sample Amazon ECS task definitions in this folder deploy the CloudWatch Agent as a Sidecar to your application to enable Amazon CloudWatch Embedded Metric Format (EMF).

* [cwagent-emf-ec2.json](cwagent-emf-ec2.json): sample ECS task definition for EC2 launch type.
* [cwagent-emf-fargate.json](cwagent-emf-fargate.json): sample ECS task definition for Fargate launch type.

The main difference between the above 2 task definitions are around the ```networkMode```: ```bridge``` vs ```awsvpc```.

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

Configure the CloudWatch agent through [SSM parameter](https://docs.aws.amazon.com/systems-manager/latest/userguide/sysman-paramstore-su-create.html). Please create a SSM parameter (```ecs-cwagent-sidecar-ec2``` or ```ecs-cwagent-sidecar-fargate```) in the region where your cluster is located, with the following text:
```
{
  "logs": {
    "metrics_collected": {
      "emf": {}
    }
  }
}
```


