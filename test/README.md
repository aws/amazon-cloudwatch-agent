# Run integration tests from your local machine

> This workflow  runs a single integration test from the local machine. It is a convenient alternate to GitHub actions; however, the limitation is that only one test can be run at a time.        

## Setup
### 1) Configure AWS on local machine
1. Create an IAM User that consumes this [inline policy](https://github.com/aws/amazon-cloudwatch-agent-test/blob/main/terraform/ec2/README.md?plain=1#L139-L278) from Terraform_Assume_Role.
   * Using an admin user also works, but it's best practice to use principle of the least privilege
2. Generate access and secret key
3. `aws configure` - [configuration basics - aws](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html)
   * Recommended to use `region=us-west-2` and `format=json`

### 2) Build binary and upload to S3
   1. Option 1: use convenience script `build_upload_binary.sh`
      1. Checkout the commit you wish to test
      2. `chmod +x ./build_upload_binary.sh`
      3. `sh ./build_upload_binary.sh`
   2. Option 2: for manual setup, read [local setup for terraform](https://github.com/aws/amazon-cloudwatch-agent-test/blob/main/terraform/ec2/README.md#local-setup-not-recommended) from the testing repo

### 3) Create `config.json`. 

`config.json` mimics the integration tests on GitHub actions. It supplies terraform with the same parameters in [integrationTest.yml](https://github.com/aws/amazon-cloudwatch-agent/blob/main/.github/workflows/integrationTest.yml).  

```json5
{
   /** REQUIRED FIELDS **/
   // relative path to the terraform suite in amazon-cloudwatch-agent-test
   "terraformRelativePath": "terraform/ec2/linux",
   // name of the s3 bucket where the agent binary is stored
   "s3Bucket": "localstack-integ-test",

   /** optional fields with default values **/
   "githubTestRepo": "https://github.com/aws/amazon-cloudwatch-agent-test.git",
   "githubTestRepoBranch": "main",
   "pluginTests": "",
   "cwaGithubSha": "${hash of commit checked out}",

   /* optional fields from matrix rows */
   "test_dir": "./test/metric_value_benchmark",
   "os": "al2",
   "family": "linux",
   "testType": "ec2_linux",
   "arc": "amd64",
   "instanceType": "t3a.medium",
   "ami": "cloudwatch-agent-integration-test-al2*",
   "binaryName": "amazon-cloudwatch-agent.rpm",
   "username": "ec2-user",
   "installAgentCommand": "go run ./install/install_agent.go rpm",
   "caCertPath": "/etc/ssl/certs/ca-bundle.crt",
   "values_per_minute": 0
}
```
* All fields are applied to terraform as an argument. Unassigned terraform vars only cause warnings
* To supply a matrix row, copy and paste from [`amazon-cloudwatch-agent-test/generator/resources`](https://github.com/aws/amazon-cloudwatch-agent-test/tree/main/generator/resources). For example, this [stress test](https://github.com/aws/amazon-cloudwatch-agent-test/blob/main/generator/resources/ec2_stress_test_matrix.json#L12-L21) ec2 matrix for al2.


### 4) Run integration test
   1. `go test -v ./test/integration` 