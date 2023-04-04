# EC2 Tagger Processor

The EC2 Tagger Processor can be used to scrape metadata from [IMDS](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) in  EC2 instances or ECS EC2 or EKS EC2 and add the metadata to the data point attributes

| Status                   |                          |
| ------------------------ |--------------------------|
| Stability                | [stable]                 |
| Supported pipeline types | metrics                  |
| Distributions            | [amazon-cloudwatch-agent]|

## Amazon Authentication

The EC2 Tagger Processor uses a credential chain for Authentication with the EC2
API endpoint. In the following order the plugin will attempt to authenticate.
1. STS Credentials if Role ARN is specified
2. Explicit credentials from 'access_key' and 'secret_key'
3. Shared profile from 'profile' (https://stackoverflow.com/a/66121705)

The next will be the default credential chain from [AWS SDK Go](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials)

4. [Environment Variables](https://github.com/aws/aws-sdk-go/wiki/configuring-sdk#environment-variables)
5. Share Credentials Files with [default profile](https://docs.aws.amazon.com/ses/latest/dg/create-shared-credentials-file.html)
6. ECS Task IAM Role
7. [EC2 Instance Profile](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html)

The IAM User or Role making the calls must have permissions to call the EC2 DescribeTags API.

### Processor Configuration:

The following receiver configuration parameters are supported.
| Name                     | Description                                                                                                    | Supported Value                          | Default | 
|--------------------------| ---------------------------------------------------------------------------------------------------------------| -----------------------------------------| --------|
|`refresh_interval_seconds`| is the frequency for the plugin to refresh the EC2 Instance Tags and ebs Volumes associated with this Instance.| "0s"                                     |   "0s"  |
|`ec2_metadata_tags`       | is the option to specify which tags to be scraped from IMDS and add to datapoint attributes                    | ["InstanceId", "ImageId", "InstanceType"]|    []   |
|`ec2_instance_tag_keys`   | is the option to specific which EC2 Instance tags to be scraped associated with this instance.                 | ["aws:autoscaling:groupName", "Name"]    |    []   |
|`disk_device_tag_key`     | is the option to Specify which tags to use to get the specified disk device name from input metric             | []                                       |    []   |

