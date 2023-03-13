# Run integration tests from your local machine

> This workflow  runs a single integration test from the local machine. It is a convenient alternate runtime to GitHub actions, but is limited to just one test at a time.      

### Setup
1. `aws configure`
2. Build binary and upload to S3
   1. Option 1: use convenience script `build_upload_binary.sh`
      1. Checkout the commit you wish to test
      2. `chmod +x ./build_upload_binary.sh`
      3. `sh ./build_upload_binary.sh`
   2. Option 2: for manual process, read [local setup for terraform](https://github.com/aws/amazon-cloudwatch-agent-test/blob/main/terraform/ec2/README.md#local-setup-not-recommended) from the testing repo
3. Create `config.json`.
   1. Option 1: use template
      1. `cp config.example.json config.json`
      2. Change S3 bucket name
   2. Option 2: manual setup
      1. Fill in general fields
         * [Required] `terraformRelativepath` - Relative path to the terraform suite in [amazon-cloudwatch-agent-test](https://github.com/aws/amazon-cloudwatch-agent-test) e.g. "terraform/ec2/linux"
         * [Required] `s3Bucket` - Name of the s3 bucket where you store your binaries for the cloudwatch agent
         * [Optional] `githubTestRepo` - Full github url to the testing repo to run against the agent binary uploaded to `s3Bucket`. Default=`https://github.com/aws/amazon-cloudwatch-agent-test.git`
         * [Optional] `githubTestRepoBranch` - Branch of `githubTestRepo` to use. Default=`main`
         * [Optional] `pluginTests` - Limits tests to a subset of plugins. Default=`""`, which tests everything 
      2. Fill in fields from a single matrix row
         * Select a single test matrix from [`amazon-cloudwatch-agent-test/generator/resources`](https://github.com/aws/amazon-cloudwatch-agent-test/tree/main/generator/resources) and append the fields. It's not necessary to run `test_case_generator.go` before grabbing these fields
4. Run integration test
   5. `go test -v ./test/integration`