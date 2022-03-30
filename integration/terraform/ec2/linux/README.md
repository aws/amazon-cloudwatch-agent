Instance assumptions

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

**How are ami built?**

AMI builder pipeline builds the ami

The pipeline installs required packages and updates ami software

This process generates a new ami we can then use for testing

**How to integration test in your aws account**
1. Create resources and setup local
   1. Install terraform
   2. Set up aws terraform user credentials
      1. User must include s3, ec2, and iam policy
      2. Currently, only us-west-2 is support so please add that to your aws config file
   3. Set up iam role for the ec2 instance
      1. Role must include CloudWatchAgentServerPolicy and s3 policy that gives both read write access
      2. Please refer to https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/create-iam-roles-for-cloudwatch-agent.html
   4. Create s3 bucket
   5. Create a key pair for ec2
      1. Must be in the region ec2 instance
   6. Hint make sure your security group allows ssh, http, and https from ipv4 all
2. Upload build/bin directory to s3
   1. make release
      1. hints
         1. You may want to do this on a linux ec2 instance installing the agent may fail if you build on Mac
         2. If you want to build faster run make build && make ${package you want ex package-rpm}
   2. aws s3 cp build/bin s3://${your bucket name}/integration-test/binary/${git commit sha} --recursive
   3. This is the agent build packages ex rpm deb
3. Start Local Stack
   1. Go to Local Stack directory
      1. cd ${path to agent dir}/integration/terraform/ec2/localstack
   2. init terraform
      1. terraform init
   3. Apply terraform
      1. ```
         terraform apply --auto-approve \
         -var="github_repo=${gh repo you want to use ex https://github.com/aws/amazon-cloudwatch-agent.git}" \
         -var="github_sha=${commit sha you want to use ex fb9229b9eaabb42461a4c049d235567f9c0439f8}" \
         -var='vpc_security_group_ids=["${name of your security group}"]' \
         -var="key_name=${name of key pair your created}" \
         -var="s3_bucket=${name of your s3 bucket created}" \
         -var="iam_instance_profile=${name of your iam role created}" \
         -var="ssh_key=${your key that you downloaded}"
         ```
      2. Write down the dns output that will be important for the next step
      3. Expected output 
         1. ```
            aws_instance.integration-test: Creation complete after 1m47s [id=i-03e33419d42b90325]

            Apply complete! Resources: 1 added, 0 changed, 0 destroyed.

            Outputs:

            public_dns = "ec2-35-87-254-148.us-west-2.compute.amazonaws.com"
            ec2-35-87-254-148.us-west-2.compute.amazonaws.com
            Completed 7.0 KiB/7.0 KiB (16.5 KiB/s) with 1 file(s) remaining
            upload: ./terraform.tfstate to s3://***/integration-test/local-stack-terraform-state/1bc666bc04255402d4516a008bb1095c5d4d27b7/terraform.tfstate
            ```
   4. Go back to linux directory
      2. cd ${path to agent dir}/integration/terraform/ec2/linux
4. Start the test linux test
   1. init terraform
      1. terraform init
   2. Apply terraform
      1. ```
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
   3. Expected Output
      1. ```
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
5. Tear down terraform state
   1. Tear down test state
      1. terraform destroy --auto-approve
   2. Go to local stack directory
      1. cd ${path to agent dir}/integration/terraform/ec2/localstack
   3. Tear down localstack state
      1. terraform destroy --auto-approve

**How To Run On Your Own Fork**
1. Follow "Create resources and setup local" except install terraform
   1. You may skip installing terraform since terraform will be installed on GitHub action runners
2. Set up GitHub action secrets in your fork
   1. Left side is the key name: right side is key value
   2. Do not wrap values in quotes
      1. This is a correct value
      2. "This is not a correct value"
   3. Must be repository secrets not environment secrets
   4. ```
        AWS_PRIVATE_KEY: ${Your private key}
        TERRAFORM_AWS_ACCESS_KEY_ID: ${User aws access key}
        TERRAFORM_AWS_SECRET_ACCESS_KEY: ${User aws secret key}
        S3_INTEGRATION_BUCKET: ${Bucket to save build}
        KEY_NAME: ${Key pair name for ec2}
        VPC_SECURITY_GROUPS_IDS: ${Security group within your vpc the value should look like ["sg-013585129c1f92bf0"]}
        IAM_ROLE: ${Role the ec2 instance should assume}
        ```