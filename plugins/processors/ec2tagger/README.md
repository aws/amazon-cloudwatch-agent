# EC2 Tagger Processor Plugin

Tags metrics with EC2 Metadata and EC2 Instance Tags.


## Amazon Authentication

This plugin uses a credential chain for Authentication with the EC2
API endpoint. In the following order the plugin will attempt to authenticate.
1. Assumed credentials via STS if `role_arn` attribute is specified (source credentials are evaluated from subsequent rules)
2. Explicit credentials from `access_key`, `secret_key`, and `token` attributes
3. Shared profile from `profile` attribute
4. [Environment Variables](https://github.com/aws/aws-sdk-go/wiki/configuring-sdk#environment-variables)
5. [Shared Credentials](https://github.com/aws/aws-sdk-go/wiki/configuring-sdk#shared-credentials-file)
6. [EC2 Instance Profile](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html)

The IAM User or Role making the calls must have permissions to call the EC2 DescribeTags API.

### Configuration:

```toml
# Configuration for adding EC2 Metadata and Instance Tags to metrics.
[[processors.ec2tagger]]
  ##
  ## ec2tagger calls AWS api to fetch EC2 Metadata and Instance Tags and EBS Volumes associated with the  
  ## current EC2 Instance and attched those values as tags to the metric.
  ## 
  ## Frequency for the plugin to refresh the EC2 Instance Tags and ebs Volumes associated with this Instance.
  ## Defaults to 0 (no refresh).
  ## When it is zero, ec2tagger doesn't do refresh to keep the ec2 tags and ebs volumes updated. However, as the
  ## AWS api request made by ec2tagger might not return the complete values (e.g. initial api call might return a 
  ## subset of ec2 tags), ec2tagger will retry every 3 minutes until all the tags/volumes (as specified by
  ## "ec2_instance_tag_keys"/"ebs_device_keys") are retrieved successfully. (Note when the specified list is ["*"],
  ## there is no way to check if all tags/volumes are retrieved, so there is no retry in that case)
  # refresh_interval_seconds = 60
  ##
  ## Add tags for EC2 Metadata fields.
  ## Supported fields are: "InstanceId", "ImageId" (aka AMI), "InstanceType"
  ## If the configuration is not provided or it has an empty list, no EC2 Metadata tags are applied.
  # ec2_metadata_tags = ["InstanceId", "ImageId", "InstanceType"]
  ##
  ## Add tags retrieved from the EC2 Instance Tags associated with this instance.
  ## If this configuration is not provided, or has an empty list, no EC2 Instance Tags are applied.
  ## If this configuration contains one entry and its value is "*", then ALL EC2 Instance Tags for the instance are applied.
  ## Note: This plugin renames the "aws:autoscaling:groupName" EC2 Instance Tag key to be spelled "AutoScalingGroupName".
  ## This aligns it with the AutoScaling dimension-name seen in AWS CloudWatch.
  # ec2_instance_tag_keys = ["aws:autoscaling:groupName", "Name"]
  ##
  ## Retrieve ebs_volume_id for the specified devices, add ebs_volume_id as tag. The specified devices are
  ## the values corresponding to the tag key "disk_device_tag_key" in the input metric.  
  ## If this configuration is not provided, or has an empty list, no ebs volume is applied.
  ## If this configuration contains one entry and its value is "*", then all ebs volume for the instance are applied.
  # ebs_device_keys = ["/dev/xvda", "/dev/nvme0n1"]
  ##
  ## Specify which tag to use to get the specified disk device name from input Metric
  # disk_device_tag_key = "device"
  ##
  ## Amazon Credentials
  ## Credentials are loaded in the following order
  ## 1) Assumed credentials via STS if role_arn is specified
  ## 2) explicit credentials from 'access_key' and 'secret_key'
  ## 3) shared profile from 'profile'
  ## 4) environment variables
  ## 5) shared credentials file
  ## 6) EC2 Instance Profile
  # access_key = ""
  # secret_key = ""
  # token = ""
  # role_arn = ""
  # profile = ""
  # shared_credential_file = ""
```

### Filters:

Processor plugins support the standard tag filter settings just like everything else.

