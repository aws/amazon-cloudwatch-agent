## CloudWatch Agent for ECS Instance Metrics Quick Start

The cloudformation template in this folder helps you to quickly deploy CloudWatch Agent as daemon-service to collect ECS Instance Metrics.

* [cwagent-ecs-instance-metric-cfn.json](cwagent-ecs-instance-metric-cfn.json): sample cloudformation template


Run following aws cloudformation command with the cloudformation template file to deploy CloudWatch Agent with required IAM roles. ***Please assign the actual ECS cluster name and the cluster region in the first two lines of the command separately.***

```
ClusterName=<your-ecs-cluster-name>
Region=<your-ecs-cluster-region>
aws cloudformation create-stack --stack-name CWAgentECS-${ClusterName}-${Region} \
    --template-body file://cwagent-ecs-instance-metric-cfn.json \
    --parameters ParameterKey=ClusterName,ParameterValue=${ClusterName} \
                 ParameterKey=CreateIAMRoles,ParameterValue=True \
    --capabilities CAPABILITY_NAMED_IAM \
    --region ${Region}
```


