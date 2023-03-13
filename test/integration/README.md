# Run integration tests from your local machine

> This workflow  runs a single integration test from the local machine. It is a convenient alternate to GitHub actions; however, the limitation is that only one test can be run at a time.        

## Setup
### 1) Configure AWS on local machine
`aws configure`

   ### 3) Build binary and upload to S3
   1. Option 1: use convenience script `build_upload_binary.sh`
      1. Checkout the commit you wish to test
      2. `chmod +x ./build_upload_binary.sh`
      3. `sh ./build_upload_binary.sh`
   2. Option 2: for manual setup, read [local setup for terraform](https://github.com/aws/amazon-cloudwatch-agent-test/blob/main/terraform/ec2/README.md#local-setup-not-recommended) from the testing repo

### 2) Create `config.json`. 

`config.json` mimics the integration tests on GitHub actions. It supplies terraform with the same parameters in [integrationTest.yml](https://github.com/aws/amazon-cloudwatch-agent/blob/main/.github/workflows/integrationTest.yml). This file is ignored by git 

```yml
   /** REQUIRED FIELDS **/
   "terraformRelativePath": "terraform/ec2/linux",
   "s3Bucket": "localstack-integ-test",

   /** optional fields **/
   // manual inputs with default values
   "githubTestRepo": "https://github.com/aws/amazon-cloudwatch-agent-test.git",
   "githubTestRepoBranch": "main",
   // default cwaGithubSha is the current commit hash checked out
   "cwaGithubSha": "a791b1484fbc0611e515ccbb9bd24bea469cb9fb",
   "pluginTests": "",

   // a single matrix row
   // copy and paste this from generator/resources
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


### 4) Run integration test
   1. `go test -v ./test/integration` based on `config.json`