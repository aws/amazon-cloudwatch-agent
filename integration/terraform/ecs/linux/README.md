Running integration tests
=========================

# Run tests in your AWS account
By running `terraform apply -auto-approve -lock=false -var="test_dir=../../../test/ecs/ecs_metadata/"`, 
you are agree to setup the following resources:
* 1 IAM Task Role and 1 Execution Task Role (similar to [these IAM Roles](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/deploy_servicelens_CloudWatch_agent_deploy_ECS.html))
* 2 SSM Parameter Store
* 2 Task Definitions and 2 Services for those task definitions
* 1 Security group which allows all inbound and outbound traffics.

After the integration test has been passed, please use the follow command to **destroy the resources**:

```terraform destroy -auto-approve ```