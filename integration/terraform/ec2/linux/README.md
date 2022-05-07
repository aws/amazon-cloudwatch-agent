Running integration tests
=========================

# Run tests in your AWS account

## Required setup

### Set up AWS credentials for Terraform

This all assumes that you are creating resources in the `us-west-2` region, as that is currently the only region that
supports the integration test AMIs.

#### Terraform IAM user permissions

For ease of use, here's a generated IAM policy based on resource usage that you can attach to your IAM user that
Terraform will assume, with the required permissions. See docs
on [Access Analyzer](https://docs.aws.amazon.com/IAM/latest/UserGuide/access-analyzer-policy-generation.html)
for how to easily generate a new policy.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:CreateTags",
        "ec2:DescribeAccountAttributes",
        "ec2:DescribeImages",
        "ec2:DescribeInstanceAttribute",
        "ec2:DescribeInstanceCreditSpecifications",
        "ec2:DescribeInstances",
        "ec2:DescribeTags",
        "ec2:DescribeVolumes",
        "ec2:DescribeVpcs",
        "ec2:GetPasswordData",
        "ec2:ModifyInstanceAttribute",
        "ec2:RunInstances",
        "ec2:TerminateInstances",
        "sts:GetCallerIdentity",
        "s3:PutObject"
      ],
      "Resource": "*"
    }
  ]
}
```

> Note: Store the IAM user key credentials in a secure location!

#### EC2 instance IAM role permissions

Refer
to [public docs](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/create-iam-roles-for-cloudwatch-agent.html)
on configuring an IAM role/policy that is required for the CloudWatch agent to function.

The EC2 instance also requires any permissions that are needed to run integration tests.

For example, if an integration test requires access to `cloudwatchlogs:GetLogEvents`, then the IAM role configured for
integration testing that gets attached to the EC2 instances must also have that permission in the policy.

For ease of use, here's a generated IAM policy based on resource usage that you can attach to your IAM role that will be
attached to the integration test EC2 instances. See docs
on [Access Analyzer](https://docs.aws.amazon.com/IAM/latest/UserGuide/access-analyzer-policy-generation.html)
for how to easily generate a new policy.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "cloudwatch:GetMetricData",
        "cloudwatch:PutMetricData",
        "cloudwatch:ListMetrics"
        "ec2:DescribeVolumes",
        "ec2:DescribeTags",
        "logs:PutLogEvents",
        "logs:DescribeLogStreams",
        "logs:DescribeLogGroups",
        "logs:CreateLogStream",
        "logs:CreateLogGroup",
        "logs:DeleteLogGroup",
        "logs:DeleteLogStream",
        "logs:PutRetentionPolicy",
        "logs:GetLogEvents",
        "logs:PutLogEvents",
        "s3:GetObjectAcl",
        "s3:GetObject",
        "s3:ListBucket"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "ssm:GetParameter"
      ],
      "Resource": "arn:aws:ssm:*:*:parameter/AmazonCloudWatch-*"
    }
  ]
}
```

### Create a test S3 bucket

See [docs](https://docs.aws.amazon.com/AmazonS3/latest/userguide/create-bucket-overview.html). The bucket does **NOT**
require public access.

### Configure security group(s)

The security group(s) that the integration tests use should include the following for ingress:

| Protocol | Port | Source    | 
|----------|------|-----------|
| TCP      | 4566 | 0.0.0.0/0 |
| HTTPS    | 443  | 0.0.0.0/0 |
| HTTP     | 80   | 0.0.0.0/0 |
| SSH      | 22   | 0.0.0.0/0 |

By default, egress allows all traffic. This is fine. 

### Create an EC2 key pair

See [docs](https://docs.aws.amazon.com/cli/latest/userguide/cli-services-ec2-keypairs.html)
on creating the key pair.
> Note: Store the private key in a secure location!

**Reminder: the EC2 key pair must be in the same region as the instances, so this assumes that the key pair is created
in the `us-west-2` region.**

## Required parameters for Terraform to have handy

1. GitHub repo (ex: https://github.com/aws/amazon-cloudwatch-agent.git)
2. GitHub SHA: `git checkout your-branch && git rev-parse --verify HEAD`
3. EC2 security groups (ex: `["sg-abc123"]`)
4. EC2 key name (the name of the `.pem` file, typically)
5. EC2 private key (the contents of the private key file)
6. IAM role **name**
    1. If you have a role ARN like `arn:aws:iam::12345:role/FooBarBaz`, then the value you want just `FooBarBaz`

## GitHub actions on your personal fork (Preferred)

The integration test GitHub actions workflow installs terraform, builds the agent and uploads the installable packages
to the configured S3 bucket, so all you need to do is configure the secrets in the GitHub repo in order to allow the
actions to run.

### Create a GPG signing key

Build artifacts get signed before being pushed out to S3. This is part of the GitHub actions workflow, so it's not
required for testing locally but is required for testing on your personal fork.

GitHub has a
good [guide](https://docs.github.com/en/authentication/managing-commit-signature-verification/generating-a-new-gpg-key)
on how to generate a new GPG key. It is good practice to create a signing key with a passphrase, and it's an expected
repository secret for the GitHub actions workflow.

> Note: Store the signing key in a secure location!

### Set up secrets on GitHub Actions

Follow [docs](https://docs.github.com/en/actions/security-guides/encrypted-secrets) on configuring GitHub Actions
secrets.

| Key                               | Description                                                                                         |
|-----------------------------------|-----------------------------------------------------------------------------------------------------|
| `AWS_PRIVATE_KEY`                 | The contents of the `.pem` file (EC2 key pair) that is used to SSH onto EC2 instances               |
| `TERRAFORM_AWS_ACCESS_KEY_ID`     | IAM user access key                                                                                 |
| `TERRAFORM_AWS_SECRET_ACCESS_KEY` | IAM user secret key                                                                                 |
| `S3_INTEGRATION_BUCKET`           | S3 bucket for dumping build artifacts                                                               |
| `KEY_NAME`                        | EC2 key pair name                                                                                   |
| `VPC_SECURITY_GROUPS_IDS`         | Security groups for the integration test EC2 instances, in the form of `["sg-abc123"]` (note `"` chars) |
| `IAM_ROLE`                        | Name of the IAM role to attach to the EC2 instances                                                 |
| `GPG_PRIVATE_KEY`                 | The contents of your GPG private key                                                                |
| `PASSPHRASE`                      | The passphrase to use for GPG signing                                                               | 
| `GPG_KEY_NAME`                    | The name of your GPG key, used as the default signing key                                           |

### Run the integration test action on your fork

1. Navigate to your fork
2. Go to `Actions`
3. Select the `Run Integration Tests` action
4. Select `Run workflow`, and choose the branch to execute integration tests on

## Local setup (Not recommended)

### Install terraform

Install `terraform` on your local machine ([download](https://www.terraform.io/downloads)).

### Build and upload agent artifacts

1. Run `make release` to test, build, and generate agent artifacts that can be installed and tested.
    1. If targeting a specific OS, you can run a more specific make command. `make build && make package-deb` would
       build and package for Ubuntu.
2. Copy the artifacts to the test S3
   bucket: `aws s3 cp ./build/bin s3://{your bucket name}/integration-test/binary/{commit SHA} --recursive`
    2. Substitute out the values wrapped in `{}` with what you have for testing

### Start localstack

Navigate to the localstack terraform directory, initialize Terraform and apply the tf plan:

```shell
cd ./integration/terraform/ec2/localstack
terraform init
terraform apply --auto-approve \
         -var="github_repo=${gh repo you want to use ex https://github.com/aws/amazon-cloudwatch-agent.git}" \
         -var="github_sha=${commit sha you want to use ex fb9229b9eaabb42461a4c049d235567f9c0439f8}" \
         -var='vpc_security_group_ids=["${name of your security group}"]' \
         -var="key_name=${name of key pair your created}" \
         -var="s3_bucket=${name of your s3 bucket created}" \
         -var="iam_instance_profile=${name of your iam role created}" \
         -var="ssh_key=${your key that you downloaded}"
```

> See the list of parameters or table of GitHub secret params as reference

Write down the public DNS output from executing the terraform plan.

Expected output:

```
aws_instance.integration-test: Creation complete after 1m47s [id=i-03e33419d42b90325]

Apply complete! Resources: 1 added, 0 changed, 0 destroyed.

Outputs:

public_dns = "ec2-35-87-254-148.us-west-2.compute.amazonaws.com"
ec2-35-87-254-148.us-west-2.compute.amazonaws.com
Completed 7.0 KiB/7.0 KiB (16.5 KiB/s) with 1 file(s) remaining
upload: ./terraform.tfstate to s3://***/integration-test/local-stack-terraform-state/1bc666bc04255402d4516a008bb1095c5d4d27b7/terraform.tfstate
```

In this example, you should keep track of `ec2-35-87-254-148.us-west-2.compute.amazonaws.com`

Start the linux integration tests:

```shell
cd ../linux # assuming you are still in the ./integration/terraform/ec2/localstack directory
terraform init
terraform apply --auto-approve \
         -var="github_repo=${gh repo you want to use ex https://github.com/aws/amazon-cloudwatch-agent.git}" \
         -var="github_sha=${commit sha you want to use ex fb9229b9eaabb42461a4c049d235567f9c0439f8}" \
         -var='vpc_security_group_ids=["${name of your security group}"]' \
         -var="s3_bucket=${name of your s3 bucket created}" \
         -var="iam_instance_profile=${name of your iam role created}" \
         -var="key_name=${name of key pair your created}" \
         -var="ami=${ami for test you want to use ex cloudwatch-agent-integration-test-ubuntu*}" \
         -var="user=${log in for the ec2 instance ex ubuntu}" \
         -var="install_agent=${command to install agent ex dpkg -i -E ./amazon-cloudwatch-agent.deb}" \
         -var="ca_cert_path=${where the default cert on the ec2 instance ex /etc/ssl/certs/ca-certificates.crt}" \
         -var="arc=${what arc to use ex amd64}" \
         -var="binary_name=${binary to install ex amazon-cloudwatch-agent.deb}" \
         -var="local_stack_host_name=${dns value you got from the local stack terraform apply step}" \
         -var="test_name=${what you want to call the ec2 instance name}" \
         -var="ssh_key=${your key that you downloaded}"
```

> See the list of parameters or table of GitHub secret params as reference

You should see tests being run on the remote hosts, like so:

```
aws_instance.integration-test (remote-exec): --- PASS: TestBundle (243.28s)
aws_instance.integration-test (remote-exec):     --- PASS: TestBundle/resource_file_location_resources/integration/ssl/with/combine/bundle_find_target_false (60.55s)
aws_instance.integration-test (remote-exec):     --- PASS: TestBundle/resource_file_location_resources/integration/ssl/without/bundle/http_find_target_false (60.55s)
aws_instance.integration-test (remote-exec):     --- PASS: TestBundle/resource_file_location_resources/integration/ssl/with/original/bundle_find_target_true (61.06s)
aws_instance.integration-test (remote-exec):     --- PASS: TestBundle/resource_file_location_resources/integration/ssl/without/bundle_find_target_true (61.13s)
aws_instance.integration-test (remote-exec): PASS
aws_instance.integration-test (remote-exec): ok  	github.com/aws/amazon-cloudwatch-agent/integration/test/ca_bundle	243.288s
aws_instance.integration-test (remote-exec): === RUN   TestEmpty
aws_instance.integration-test (remote-exec): --- PASS: TestEmpty (0.00s)
aws_instance.integration-test (remote-exec): PASS
aws_instance.integration-test (remote-exec): ok  	github.com/aws/amazon-cloudwatch-agent/integration/test/empty	0.002s
aws_instance.integration-test (remote-exec): === RUN   TestAgentStatus
aws_instance.integration-test: Still creating... [5m30s elapsed]
aws_instance.integration-test (remote-exec): --- PASS: TestAgentStatus (6.54s)
aws_instance.integration-test (remote-exec): PASS
aws_instance.integration-test (remote-exec): ok  	github.com/aws/amazon-cloudwatch-agent/integration/test/sanity	6.541s
aws_instance.integration-test: Creation complete after 5m35s [id=i-0f7f77a62c93df010]

Apply complete! Resources: 1 added, 0 changed, 0 destroyed.   
```

After running tests, tear down everything with Terraform:

```shell
# assuming still in the ./integration/terraform/ec2/linux directory
terraform destroy --auto-approve
cd ../localstack
terraform destroy --auto-approve
```

# How are AMIs built?

1. AMI builder pipeline builds the ami
2. The pipeline installs required packages and updates ami software
3. This process generates a new ami we can then use for testing

## Instance software assumptions

1. docker
    1. starts on start up
    2. does not require sudo
2. docker-compose
3. golang
4. openssl
5. git
6. make
7. aws-cli
8. CloudWatchAgentServerRole is attached
9. crontab
10. gcc
11. python3
