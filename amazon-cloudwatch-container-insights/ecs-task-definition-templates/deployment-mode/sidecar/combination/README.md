## CloudWatch Agent multi-feature deployments

* [combination-ec2.json](combination-ec2.json) provides an example for the EC2 launch type for deploying the agent as a Sidecar to set up all the individual features.

* [combination-fargate.json](combination-fargate.json) provides an example for the Fargate launch type for deploying the agent as a Sidecar to set up all the individual features.


The main difference between the above 2 task definitions are around the ```networkMode```: ```bridge``` vs ```awsvpc```.

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
  "metrics": {
    "metrics_collected": {
      "statsd": {
        "service_address":":8125"
      }
    }
  },
  "logs": {
    "metrics_collected": {
      "emf": {}
    }
  },
  "csm": {
    "service_addresses": ["udp4://0.0.0.0:31000", "udp6://[::1]:31000"],
    "memory_limit_in_mb": 20
  }
}
```

If you are launching an ECS task for the Fargate launch type, please create a SSM parameter ```ecs-cwagent-sidecar-fargate``` in the region where your cluster is located, with the following text:
```
{
  "metrics": {
    "metrics_collected": {
      "statsd": {
        "service_address":":8125"
      }
    }
  },
  "logs": {
    "metrics_collected": {
      "emf": {}
    }
  },
  "csm": {
    "service_addresses": ["udp4://127.0.0.1:31000", "udp6://[::1]:31000"],
    "memory_limit_in_mb": 20
  }
}
```
