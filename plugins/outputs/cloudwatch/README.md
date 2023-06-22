## Amazon CloudWatch Exporter for Open Telemetry

The AmazonCloudWatch Exporter will convert the OTEL metrics to MetricDatum and send them to Amazon CloudWatch 

| Status                   |                          |
| ------------------------ |--------------------------|
| Stability                | [stable]                 |
| Supported pipeline types | metrics                  |
| Distributions            | [amazon-cloudwatch-agent]|

## Amazon Authentication

The AmazonCloudWatch Exporter uses a credential chain for Authentication with the EC2
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

### Exporter Configuration:

The following receiver configuration parameters are supported.
| Name                     | Description                                                                                                    | Default    | 
|--------------------------| ---------------------------------------------------------------------------------------------------------------| -----------|
|`region`                  | is the Amazon region that you wish to connect to. (e.g us-west-2, us-west-2)                                   | ""         |
|`namespace`               | is the namespace used for AWS CloudWatch metrics.                                                              | "CWAgent   |
|`endpoint_override`       | is the endpoint you want to use other than the default endpoint based on the region information.               | ""         |
