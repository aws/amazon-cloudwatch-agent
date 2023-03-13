# Run integration tests from your local machine

> This workflow  runs a single integration test from the local machine. It is a convenient alternate runtime to GitHub actions, but is limited to just one test at a time.      

### Setup
1. Build binary and upload to S3
   1. A convenience script has been written for you
      1. Checkout the commit you wish to test
      2. `chmod +x ./build_upload_binary.sh`
      3. `sh ./build_upload_binary.sh`
   2. For more information, read the [local setup for ec2 terraform section](https://github.com/aws/amazon-cloudwatch-agent-test/blob/main/terraform/ec2/README.md#local-setup-not-recommended) the test repo
2. `aws configure`
3. Create `config.json`. 
An example json file has already been created for you. This file is ignored by git to protect your secrets
   1. Fill in required fields
   2. Fill in a matrix row
4. `go test -v ./test/integration`