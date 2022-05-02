Running ECS Fargate Integration Tests
=========================

# 1. Setup resources
By running `terraform apply -auto-approve -lock=false`, 
you agree to setup the following resources:
* 1 IAM Task Role and 1 Execution Task Role (similar to [these IAM Roles](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/deploy_servicelens_CloudWatch_agent_deploy_ECS.html))
* 2 SSM Parameter Store
* 2 Task Definitions and 2 Services for those task definitions
* 1 Security group which allows all inbound and outbound traffics.

To be more specifically,
* **IAM Task Role:** Contain the following policy
  * **CloudWatchAgentPolicy:** CloudWatchAgent's related actions
  * **service_discovery_police:** For describe ECS tasks and services
```json
{
  "Statement": [
    {
      "Action": [
        "ecs:ListTasks",
        "ecs:ListServices",
        "ecs:DescribeTasks",
        "ecs:DescribeTaskDefinition",
        "ecs:DescribeServices",
        "ecs:DescribeContainerInstances",
        "ec2:DescribeInstances"
      ],
      "Effect": "Allow",
      "Resource": "*",
      "Sid": ""
    }
  ],
  "Version": "2012-10-17"
}
```
  
* **IAM Execution Task Role:** Contain the following policy
  * **AmazonECSTaskExecutionRolePolicy:** Pull CloudWatch Agent's image and extra app's image from ECR.
  * **AmazonSSMReadOnlyAccess:** Pull Cloudwatch Agent's and Prometheus's config  from SSM Parameter Store.
* **CloudWatchAgent Parameter Store:** Store CloudWatchAgent's configuration and CloudWatchAgent will pull the config from there. [Example configuration](default_resources/default_amazon_cloudwatch_agent.json)
* **Prometheus Parameter Store:** Store Prometheus's configuration and CloudWatchAgent will pull the config from there. [Example configuration](default_resources/default_ecs_prometheus.tpl)

# 2. Run tests in your AWS account
````
cd terraform/ecs && terraform init && terraform apply -auto-approve \
    -var="test_dir=../../../test/ecs/ecs_metadata/ \
    -var="aoc_version={{the aoc binary version}}" \
    -var="testcase=../testcases/{{your test case folder name}}" \
    -var-file="../testcases/{{your test case folder name}}/parameters.tfvars"
````

Don't forget to clean up your resources:
````
terraform destroy -auto-approve
````