Running ECS Fargate Integration Tests
=========================

## 1. How ECS Fargate are set up?
**Step 1:** Create a Fargate ECS Cluster with the default VPC Network.   
**Step 2:** Create a security group to assign to the service in step 5 which allows all inbound 
traffics and outbound traffics    
**Step 3:** Create a IAM Role and IAM Execution Role for the containers to pull the image and 
execute their purposes  
**Step 4:** Create a [task definition](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definitions.html) 
to decide which containers serve a specific task   and assign the IAM roles in step 3 to the containers   
**Step 5:** Create a [service](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_services.html) which configure 
how many tasks are running in parallel and ensure availability of the task. 

## 2. Setup resources
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

## 3. Run tests in your AWS account
````
cd integration/terraform/ecs && terraform init && terraform apply -auto-approve \
    -var="test_dir={{your test case folder name}} \
````

Don't forget to clean up your resources after integration test has passed:
````
terraform destroy -auto-approve
````